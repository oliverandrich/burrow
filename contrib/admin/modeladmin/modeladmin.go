// Package modeladmin provides a generic, Django-style ModelAdmin for
// auto-generating CRUD admin views from Bun models.
package modeladmin

import (
	"context"
	"html/template"
	"net/http"
	"reflect"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/forms"
	"github.com/oliverandrich/burrow/i18n"
)

// ChoicesFunc returns dynamic choices for a select field, typically loaded from the database.
type ChoicesFunc func(ctx context.Context) ([]forms.Choice, error)

const defaultPageSize = 25

// ModelAdmin provides generic CRUD admin views for a Bun model.
// It is not a burrow.App itself — it's a helper that apps embed
// to delegate admin route handling.
type ModelAdmin[T any] struct { //nolint:govet // fieldalignment: readability over optimization
	// Slug is the URL path segment, e.g. "articles".
	Slug string
	// DisplayName is the singular human-readable name, also used as i18n key, e.g. "Article".
	DisplayName string
	// DisplayPluralName is the plural human-readable name, also used as i18n key, e.g. "Articles".
	DisplayPluralName string
	// DB is the Bun database connection.
	DB *bun.DB
	// Renderer renders list, detail, and form views.
	Renderer Renderer[T]
	// IDFunc extracts the primary key from the request URL.
	// Defaults to chi.URLParam(r, "id").
	IDFunc func(*http.Request) string
	// CanCreate enables the create action. Default: false.
	CanCreate bool
	// CanEdit enables the update action. Default: false.
	CanEdit bool
	// CanDelete enables the delete action. Default: false.
	CanDelete bool
	// ListFields lists struct field names to show in the list view.
	ListFields []string
	// OrderBy is the default ORDER BY clause, e.g. "created_at DESC".
	OrderBy string
	// Relations lists Bun .Relation() names to eager-load.
	Relations []string
	// PageSize is the number of items per page. Default: 25.
	PageSize int
	// SearchFields lists database column names to full-text search across.
	SearchFields []string
	// Filters defines the sidebar filters for the list view.
	Filters []FilterDef
	// SortFields lists database column names that support column sorting.
	SortFields []string
	// RowActions defines custom per-row actions for the list/detail views.
	RowActions []RowAction
	// FieldChoices provides dynamic choices for form select fields.
	// Key is the Go struct field name, value is a function that returns choices
	// (typically loaded from the database). Fields with FieldChoices are
	// rendered as <select> dropdowns instead of text/number inputs.
	FieldChoices map[string]ChoicesFunc
	// EmptyMessage is shown when the list has no items. Default: "No items found."
	EmptyMessage string
	// EmptyMessageKey is the i18n key for the empty-list message.
	// Translated via i18n.T at request time.
	EmptyMessageKey string
	// ReadOnlyFields lists struct field names (by Go name) to render as
	// plain text in forms. Read-only fields cannot be modified by the user;
	// their values are preserved from the model instance.
	ReadOnlyFields []string
	// CanExport enables CSV/JSON export of the list view. Default: false.
	CanExport bool
	// ListDisplay defines computed columns for the list view.
	// Keys are column names (which can also appear in ListFields).
	// The function receives an item and returns pre-rendered HTML.
	ListDisplay map[string]func(T) template.HTML

	// ftsTable is the detected FTS5 table name (e.g. "notes_fts").
	// Set automatically in Routes() if a {tablename}_fts table exists.
	ftsTable string
}

// idFromRequest returns the ID from the URL, using IDFunc if set.
func (ma *ModelAdmin[T]) idFromRequest(r *http.Request) string {
	if ma.IDFunc != nil {
		return ma.IDFunc(r)
	}
	return chi.URLParam(r, "id")
}

// pageSize returns the configured page size or the default.
func (ma *ModelAdmin[T]) pageSize() int {
	if ma.PageSize > 0 {
		return ma.PageSize
	}
	return defaultPageSize
}

// Renderer defines how ModelAdmin views are rendered.
type Renderer[T any] interface {
	List(w http.ResponseWriter, r *http.Request, items []T, page burrow.PageResult, cfg RenderConfig) error
	Detail(w http.ResponseWriter, r *http.Request, item *T, cfg RenderConfig) error
	Form(w http.ResponseWriter, r *http.Request, item *T, fields []forms.BoundField, cfg RenderConfig) error
	ConfirmDelete(w http.ResponseWriter, r *http.Request, item *T, cfg RenderConfig) error
}

// ActiveFilter holds filter state for rendering in the list template.
type ActiveFilter struct {
	Field     string
	Label     string
	Choices   []ActiveChoice
	HasActive bool // true if any choice is selected
}

// ActiveChoice represents a single filter option with its computed URL.
type ActiveChoice struct {
	Value    string
	Label    string
	URL      string // pre-built URL with this filter applied
	IsActive bool
}

// RenderConfig holds display metadata passed to the renderer.
type RenderConfig struct { //nolint:govet // fieldalignment: readability over optimization
	Slug              string
	DisplayName       string
	DisplayPluralName string
	CanCreate         bool
	CanEdit           bool
	CanDelete         bool
	CanExport         bool
	ListFields        []string // Go struct field names (for columnValue/fieldValue lookups)
	ListFieldLabels   []string // translated column headers (parallel to ListFields)
	IDField           string   // struct field name for the primary key (default: "ID")
	Filters           []ActiveFilter
	RowActions        []RenderAction
	HasRowActions     bool
	ItemActionSets    [][]RenderAction // per-item action sets, parallel to items (ShowWhen-evaluated)
	EmptyMessage      string
	ComputedColumns   map[string]func(any) template.HTML // field → render function
}

// computedColumns type-erases ListDisplay into a map usable by columnHTML.
func (ma *ModelAdmin[T]) computedColumns() map[string]func(any) template.HTML {
	if len(ma.ListDisplay) == 0 {
		return nil
	}
	m := make(map[string]func(any) template.HTML, len(ma.ListDisplay))
	for name, fn := range ma.ListDisplay {
		m[name] = func(item any) template.HTML {
			typed, ok := item.(T) //nolint:errcheck // type is guaranteed by ModelAdmin[T]
			if !ok {
				return "<span>-</span>"
			}
			return fn(typed)
		}
	}
	return m
}

// renderConfig returns the RenderConfig for this ModelAdmin.
func (ma *ModelAdmin[T]) renderConfig() RenderConfig {
	idField := "ID"
	if f := pkFieldName[T](); f != "" {
		idField = f
	}
	renderActions := make([]RenderAction, 0, len(ma.RowActions))
	for _, a := range ma.RowActions {
		renderActions = append(renderActions, a.toRenderAction())
	}
	emptyMsg := ma.EmptyMessage
	if emptyMsg == "" {
		emptyMsg = "No items found."
	}
	return RenderConfig{
		Slug:              ma.Slug,
		DisplayName:       ma.DisplayName,
		DisplayPluralName: ma.DisplayPluralName,
		CanCreate:         ma.CanCreate,
		CanEdit:           ma.CanEdit,
		CanDelete:         ma.CanDelete,
		CanExport:         ma.CanExport,
		ListFields:        ma.ListFields,
		IDField:           idField,
		RowActions:        renderActions,
		HasRowActions:     len(renderActions) > 0,
		EmptyMessage:      emptyMsg,
	}
}

// translateRenderConfig applies request-time i18n translations to the render config.
func (ma *ModelAdmin[T]) translateRenderConfig(cfg *RenderConfig, r *http.Request) {
	ctx := r.Context()

	// Translate verbose names as column labels.
	vnames := verboseNames[T]()
	labels := make([]string, len(cfg.ListFields))
	for i, f := range cfg.ListFields {
		if vn, ok := vnames[f]; ok {
			labels[i] = i18n.T(ctx, vn)
		} else {
			labels[i] = f
		}
	}
	cfg.ListFieldLabels = labels

	// Translate display names.
	if ma.DisplayName != "" {
		cfg.DisplayName = i18n.T(ctx, ma.DisplayName)
	}
	if ma.DisplayPluralName != "" {
		cfg.DisplayPluralName = i18n.T(ctx, ma.DisplayPluralName)
	}
	if ma.EmptyMessageKey != "" {
		cfg.EmptyMessage = i18n.T(ctx, ma.EmptyMessageKey)
	}
}

// formOptions builds forms.Option[T] from bun tag analysis and FieldChoices.
// Choices are eagerly resolved from FieldChoices using the given context.
func (ma *ModelAdmin[T]) formOptions(ctx context.Context) ([]forms.Option[T], error) {
	var opts []forms.Option[T]

	// Exclude autoincrement PKs.
	if excluded := bunAutoIncrementPKs[T](); len(excluded) > 0 {
		opts = append(opts, forms.WithExclude[T](excluded...))
	}

	// Mark read-only fields.
	if len(ma.ReadOnlyFields) > 0 {
		opts = append(opts, forms.WithReadOnly[T](ma.ReadOnlyFields...))
	}

	// Eagerly resolve FieldChoices to static choices.
	for field, fn := range ma.FieldChoices {
		choices, err := fn(ctx)
		if err != nil {
			return nil, err
		}
		opts = append(opts, forms.WithChoices[T](field, choices))
	}

	return opts, nil
}

// bunAutoIncrementPKs returns field names tagged with bun:",pk,autoincrement".
func bunAutoIncrementPKs[T any]() []string {
	t := reflect.TypeFor[T]()
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	var result []string
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() || sf.Anonymous {
			continue
		}
		bunTag := sf.Tag.Get("bun")
		if containsOption(bunTag, "pk") && containsOption(bunTag, "autoincrement") {
			result = append(result, sf.Name)
		}
	}
	return result
}

// ColumnValue extracts a display value for a list column from an item.
func ColumnValue(item any, field string) template.HTML {
	return columnHTML(item, field, nil, nil)
}

// ColumnValueFunc returns a columnValue function that uses the given translator
// and computed column functions.
func ColumnValueFunc(t func(string) string, computed map[string]func(any) template.HTML) func(any, string) template.HTML {
	return func(item any, field string) template.HTML {
		return columnHTML(item, field, t, computed)
	}
}
