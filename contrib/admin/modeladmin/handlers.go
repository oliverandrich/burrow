package modeladmin

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/htmx"
)

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
		fields := AutoFields[T](item)
		translateFormFields(fields, r)
		return ma.Renderer.Form(w, r, item, fields, nil, cfg)
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
	fields := AutoFields[T](nil)
	translateFormFields(fields, r)
	return ma.Renderer.Form(w, r, nil, fields, nil, cfg)
}

// HandleCreate processes the create form submission.
// Exported so apps can mount ModelAdmin create alongside custom handlers.
func (ma *ModelAdmin[T]) HandleCreate(w http.ResponseWriter, r *http.Request) error {
	if !ma.CanCreate {
		return burrow.NewHTTPError(http.StatusForbidden, "create not allowed")
	}

	item := new(T)
	if err := PopulateFromForm(r, item); err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid form data")
	}

	if err := burrow.Validate(item); err != nil {
		var ve *burrow.ValidationError
		if errors.As(err, &ve) {
			vCfg := ma.renderConfig()
			ma.translateRenderConfig(&vCfg, r)
			fields := AutoFields[T](item)
			translateFormFields(fields, r)
			return ma.Renderer.Form(w, r, item, fields, ve, vCfg)
		}
		return burrow.NewHTTPError(http.StatusBadRequest, "validation failed")
	}

	if err := createItem(r.Context(), ma.DB, item); err != nil {
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

	// Verify item exists.
	item, err := getItem[T](r.Context(), ma.DB, id, ma.Relations)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return burrow.NewHTTPError(http.StatusNotFound, "item not found")
		}
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to get item")
	}

	if err := PopulateFromForm(r, item); err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid form data")
	}

	if err := burrow.Validate(item); err != nil {
		var ve *burrow.ValidationError
		if errors.As(err, &ve) {
			vCfg := ma.renderConfig()
			ma.translateRenderConfig(&vCfg, r)
			fields := AutoFields[T](item)
			translateFormFields(fields, r)
			return ma.Renderer.Form(w, r, item, fields, ve, vCfg)
		}
		return burrow.NewHTTPError(http.StatusBadRequest, "validation failed")
	}

	if err := updateItem(r.Context(), ma.DB, item); err != nil {
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
