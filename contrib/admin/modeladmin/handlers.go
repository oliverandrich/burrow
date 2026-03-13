package modeladmin

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/burrow/forms"
	"github.com/oliverandrich/burrow/i18n"
)

// HandleBulkAction processes a bulk action on selected items.
func (ma *ModelAdmin[T]) HandleBulkAction(w http.ResponseWriter, r *http.Request) error {
	slug := chi.URLParam(r, "action")

	var action *BulkAction
	for i := range ma.BulkActions {
		if ma.BulkActions[i].Slug == slug {
			action = &ma.BulkActions[i]
			break
		}
	}
	if action == nil {
		return burrow.NewHTTPError(http.StatusNotFound, "unknown bulk action")
	}

	if err := r.ParseForm(); err != nil { //nolint:gosec // G120: body size limited by server-level RequestSize middleware
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid form data")
	}
	ids := r.Form["_selected"]
	if len(ids) == 0 {
		return burrow.NewHTTPError(http.StatusBadRequest, "no items selected")
	}

	if err := action.Handler(r.Context(), ma.DB, ids); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "bulk action failed")
	}

	slog.Info("bulk action executed", "slug", ma.Slug, "action", slug, "count", len(ids)) //nolint:gosec // slug is developer-set
	_ = messages.AddSuccess(w, r, i18n.TPlural(r.Context(), "modeladmin-bulk-success", len(ids)))
	http.Redirect(w, r, "/admin/"+ma.Slug, http.StatusSeeOther)
	return nil
}

// HandleList renders the paginated list view.
// Exported so apps can mount ModelAdmin list views alongside custom handlers.
func (ma *ModelAdmin[T]) HandleList(w http.ResponseWriter, r *http.Request) error {
	pr := burrow.ParsePageRequest(r)
	if pr.Limit == 0 || pr.Limit > ma.pageSize() {
		pr.Limit = ma.pageSize()
	}
	if pr.Page == 0 {
		pr.Page = 1
	}

	opts := listOpts{
		relations:    ma.Relations,
		orderBy:      ma.OrderBy,
		searchTerm:   r.URL.Query().Get("q"),
		searchFields: ma.SearchFields,
		ftsTable:     ma.ftsTable,
		filters:      ma.Filters,
		sortFields:   ma.SortFields,
		r:            r,
	}
	items, page, err := listItems[T](r.Context(), ma.DB, opts, pr)
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list items")
	}

	cfg := ma.renderConfig()
	ma.translateRenderConfig(&cfg, r)
	cfg.Filters = buildActiveFilters(ma.Filters, r)
	cfg.ComputedColumns = ma.computedColumns()
	if cfg.HasRowActions {
		cfg.ItemActionSets = make([][]RenderAction, len(items))
		for i, item := range items {
			cfg.ItemActionSets[i] = buildItemActions(ma.RowActions, item).Actions
		}
	}
	return ma.Renderer.List(w, r, items, page, cfg)
}

// HandleDetail renders the detail/edit form for an existing item.
// Exported so apps can mount ModelAdmin detail views alongside custom handlers.
func (ma *ModelAdmin[T]) HandleDetail(w http.ResponseWriter, r *http.Request) error {
	id := ma.idFromRequest(r)
	if id == "" {
		return burrow.NewHTTPError(http.StatusBadRequest, "missing id")
	}

	item, err := getItem[T](r.Context(), ma.DB, id, ma.Relations)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return burrow.NewHTTPError(http.StatusNotFound, "item not found")
		}
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to get item")
	}

	cfg := ma.renderConfig()
	ma.translateRenderConfig(&cfg, r)
	if cfg.HasRowActions {
		cfg.ItemActionSets = [][]RenderAction{
			buildItemActions(ma.RowActions, *item).Actions,
		}
	}

	if ma.CanEdit {
		opts, err := ma.formOptions(r.Context())
		if err != nil {
			return burrow.NewHTTPError(http.StatusInternalServerError, "failed to load field choices")
		}
		f := forms.FromModel(item, opts...)
		fields := f.Fields()
		translateBoundFields(fields, r)
		return ma.Renderer.Form(w, r, item, fields, cfg)
	}

	return ma.Renderer.Detail(w, r, item, cfg)
}

// HandleNew renders the create form.
// Exported so apps can mount ModelAdmin form views alongside custom handlers.
func (ma *ModelAdmin[T]) HandleNew(w http.ResponseWriter, r *http.Request) error {
	if !ma.CanCreate {
		return burrow.NewHTTPError(http.StatusForbidden, "create not allowed")
	}

	cfg := ma.renderConfig()
	ma.translateRenderConfig(&cfg, r)
	opts, err := ma.formOptions(r.Context())
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to load field choices")
	}
	f := forms.FromModel[T](nil, opts...)
	fields := f.Fields()
	translateBoundFields(fields, r)
	return ma.Renderer.Form(w, r, nil, fields, cfg)
}

// HandleCreate processes the create form submission.
// Exported so apps can mount ModelAdmin create alongside custom handlers.
func (ma *ModelAdmin[T]) HandleCreate(w http.ResponseWriter, r *http.Request) error {
	if !ma.CanCreate {
		return burrow.NewHTTPError(http.StatusForbidden, "create not allowed")
	}

	opts, err := ma.formOptions(r.Context())
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to load field choices")
	}
	f := forms.FromModel[T](nil, opts...)
	if !f.Bind(r) {
		vCfg := ma.renderConfig()
		ma.translateRenderConfig(&vCfg, r)
		fields := f.Fields()
		translateBoundFields(fields, r)
		return ma.Renderer.Form(w, r, f.Instance(), fields, vCfg)
	}

	if err := createItem(r.Context(), ma.DB, f.Instance()); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to create item")
	}

	slog.Info("item created", "slug", ma.Slug) //nolint:gosec // slug is set by developer, not user input
	http.Redirect(w, r, "/admin/"+ma.Slug, http.StatusSeeOther)
	return nil
}

// HandleUpdate processes the edit form submission.
// Exported so apps can mount ModelAdmin update alongside custom handlers.
func (ma *ModelAdmin[T]) HandleUpdate(w http.ResponseWriter, r *http.Request) error {
	if !ma.CanEdit {
		return burrow.NewHTTPError(http.StatusForbidden, "edit not allowed")
	}

	id := ma.idFromRequest(r)
	if id == "" {
		return burrow.NewHTTPError(http.StatusBadRequest, "missing id")
	}

	// Verify item exists and use it as the form base.
	item, err := getItem[T](r.Context(), ma.DB, id, ma.Relations)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return burrow.NewHTTPError(http.StatusNotFound, "item not found")
		}
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to get item")
	}

	opts, err := ma.formOptions(r.Context())
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to load field choices")
	}
	f := forms.FromModel(item, opts...)
	if !f.Bind(r) {
		vCfg := ma.renderConfig()
		ma.translateRenderConfig(&vCfg, r)
		fields := f.Fields()
		translateBoundFields(fields, r)
		return ma.Renderer.Form(w, r, f.Instance(), fields, vCfg)
	}

	if err := updateItem(r.Context(), ma.DB, f.Instance()); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to update item")
	}

	slog.Info("item updated", "slug", ma.Slug, "id", id) //nolint:gosec // slug is developer-set, id is from URL param

	redirectURL := "/admin/" + ma.Slug
	if r.FormValue("_continue") != "" { //nolint:gosec // G120: body size limited by server-level RequestSize middleware
		redirectURL = "/admin/" + ma.Slug + "/" + id
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	return nil
}

// HandleConfirmDelete renders the delete confirmation page.
// Exported so apps can mount ModelAdmin confirm-delete views alongside custom handlers.
func (ma *ModelAdmin[T]) HandleConfirmDelete(w http.ResponseWriter, r *http.Request) error {
	if !ma.CanDelete {
		return burrow.NewHTTPError(http.StatusForbidden, "delete not allowed")
	}

	id := ma.idFromRequest(r)
	if id == "" {
		return burrow.NewHTTPError(http.StatusBadRequest, "missing id")
	}

	item, err := getItem[T](r.Context(), ma.DB, id, ma.Relations)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return burrow.NewHTTPError(http.StatusNotFound, "item not found")
		}
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to get item")
	}

	cfg := ma.renderConfig()
	ma.translateRenderConfig(&cfg, r)

	if len(ma.cascades) > 0 {
		impacts, err := countCascadeImpacts(r.Context(), ma.DB, ma.cascades, id)
		if err != nil {
			slog.Error("failed to count cascade impacts", "error", err, "slug", ma.Slug, "id", id) //nolint:gosec // slug is developer-set, id is from URL param
		} else {
			cfg.DeleteImpacts = impacts
		}
	}

	return ma.Renderer.ConfirmDelete(w, r, item, cfg)
}

// HandleDelete deletes an item by ID.
// Exported so apps can mount ModelAdmin delete alongside custom handlers.
func (ma *ModelAdmin[T]) HandleDelete(w http.ResponseWriter, r *http.Request) error {
	if !ma.CanDelete {
		return burrow.NewHTTPError(http.StatusForbidden, "delete not allowed")
	}

	id := ma.idFromRequest(r)
	if id == "" {
		return burrow.NewHTTPError(http.StatusBadRequest, "missing id")
	}

	// Verify item exists.
	if _, err := getItem[T](r.Context(), ma.DB, id, ma.Relations); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return burrow.NewHTTPError(http.StatusNotFound, "item not found")
		}
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to get item")
	}

	if err := deleteItem[T](r.Context(), ma.DB, id); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to delete item")
	}

	slog.Info("item deleted", "slug", ma.Slug, "id", id) //nolint:gosec // slug is developer-set, id is from URL param
	htmx.Redirect(w, "/admin/"+ma.Slug)
	w.WriteHeader(http.StatusOK)
	return nil
}

// translateBoundFields translates field labels via i18n at request time.
func translateBoundFields(fields []forms.BoundField, r *http.Request) {
	ctx := r.Context()
	for i := range fields {
		fields[i].Label = i18n.T(ctx, fields[i].Label)
	}
}
