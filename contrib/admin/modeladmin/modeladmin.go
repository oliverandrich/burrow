// Package modeladmin provides a generic, Django-style ModelAdmin for
// auto-generating CRUD admin views from Bun models.
package modeladmin

import (
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"codeberg.org/oliverandrich/burrow"
)

const defaultPageSize = 25

// ModelAdmin provides generic CRUD admin views for a Bun model.
// It is not a burrow.App itself — it's a helper that apps embed
// to delegate admin route handling.
type ModelAdmin[T any] struct { //nolint:govet // fieldalignment: readability over optimization
	// Slug is the URL path segment, e.g. "articles".
	Slug string
	// Display is the human-readable plural name, e.g. "Articles".
	Display string
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
	// EmptyMessage is shown when the list has no items. Default: "No items found."
	EmptyMessage string
	// EmptyMessageKey is the i18n key for the empty-list message.
	// Translated via TranslateFunc when set.
	EmptyMessageKey string
	// TranslateFunc translates i18n keys at request time. Used for filter labels,
	// choice labels, and other UI strings with LabelKey fields set.
	TranslateFunc func(r *http.Request, key string) string
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
	Form(w http.ResponseWriter, r *http.Request, item *T, fields []FormField, errors *burrow.ValidationError, cfg RenderConfig) error
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
	Slug           string
	Display        string
	CanCreate      bool
	CanEdit        bool
	CanDelete      bool
	ListFields     []string
	IDField        string // struct field name for the primary key (default: "ID")
	Filters        []ActiveFilter
	RowActions     []RenderAction
	HasRowActions  bool
	ItemActionSets [][]RenderAction // per-item action sets, parallel to items (ShowWhen-evaluated)
	EmptyMessage   string
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
		Slug:          ma.Slug,
		Display:       ma.Display,
		CanCreate:     ma.CanCreate,
		CanEdit:       ma.CanEdit,
		CanDelete:     ma.CanDelete,
		ListFields:    ma.ListFields,
		IDField:       idField,
		RowActions:    renderActions,
		HasRowActions: len(renderActions) > 0,
		EmptyMessage:  emptyMsg,
	}
}

// ColumnValue extracts a display value for a list column from an item.
func ColumnValue(item any, field string) template.HTML {
	return columnHTML(item, field)
}
