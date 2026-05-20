package notes

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/burrow/forms"
	"github.com/oliverandrich/burrow/i18n"
)

// All handlers in this file are behind RequireAuth middleware,
// so MustCurrentUser is safe to use throughout.

// noteFormOpts returns the common form options for the Note form.
func noteFormOpts() []forms.Option[Note] {
	return []forms.Option[Note]{
		forms.WithExclude[Note]("ID", "UserID", "CreatedAt"),
	}
}

// List renders the user's notes as an HTML page with offset-based pagination.
func (a *App) List(w http.ResponseWriter, r *http.Request) error {
	user := auth.MustCurrentUser[Profile](r.Context())

	pr := burrow.ParsePageRequest(r)
	searchQuery := r.URL.Query().Get("q")

	var notes []Note
	var page burrow.PageResult
	var err error

	if searchQuery != "" {
		notes, page, err = a.repo.SearchByUserID(r.Context(), user.ID, searchQuery, pr)
	} else {
		notes, page, err = a.repo.ListByUserIDPaged(r.Context(), user.ID, pr)
	}
	if err != nil {
		return err
	}

	data := map[string]any{
		"Notes":       notes,
		"Page":        page,
		"Title":       "Notes",
		"SearchQuery": searchQuery,
	}

	tmpl := "notes/list_page"
	if htmx.Request(r).IsHTMX() {
		switch {
		case pr.Page > 1:
			tmpl = "notes/notes_page"
		case r.URL.Query().Has("q"):
			tmpl = "notes/notes_list"
		}
	}

	return burrow.Render(w, r, http.StatusOK, tmpl, data)
}

// New renders the empty create form into the modal dialog.
func (a *App) New(w http.ResponseWriter, r *http.Request) error {
	_ = auth.MustCurrentUser[Profile](r.Context())

	f := forms.New[Note](noteFormOpts()...)
	data := map[string]any{
		"Fields":   f.Fields(),
		"TitleKey": "notes-new-title",
		"Action":   "/notes",
	}
	htmx.OpenDialog(w, "modal")
	return burrow.Render(w, r, http.StatusOK, "notes/form", data)
}

// Create adds a new note for the authenticated user.
func (a *App) Create(w http.ResponseWriter, r *http.Request) error {
	user := auth.MustCurrentUser[Profile](r.Context())

	f := forms.New[Note](noteFormOpts()...)
	if !f.Bind(r) {
		return burrow.Render(w, r, http.StatusUnprocessableEntity, "notes/form", map[string]any{
			"Fields":         f.Fields(),
			"NonFieldErrors": f.NonFieldErrors(),
			"TitleKey":       "notes-new-title",
			"Action":         "/notes",
		})
	}

	note := f.Instance()
	note.UserID = user.ID

	if err := a.repo.Create(r.Context(), note); err != nil {
		return err
	}

	if err := messages.AddSuccess(w, r, i18n.T(r.Context(), "notes-created")); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to add flash message")
	}

	// HTMX: prepend new card via OOB + close the dialog.
	htmx.CloseDialog(w, "modal")
	return htmx.RenderOrRedirect(w, r, "/notes", "notes/create_response", map[string]any{
		"Note":     note,
		"Messages": messages.Get(r.Context()),
	})
}

// Edit renders the edit form pre-filled with an existing note into the modal.
func (a *App) Edit(w http.ResponseWriter, r *http.Request) error {
	user := auth.MustCurrentUser[Profile](r.Context())
	id := chi.URLParam(r, "id")

	note, err := a.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return burrow.NewHTTPError(http.StatusNotFound, "note not found")
		}
		return err
	}

	f := forms.FromModel(note, noteFormOpts()...)
	data := map[string]any{
		"Fields":   f.Fields(),
		"TitleKey": "notes-edit-title",
		"Action":   "/notes/" + note.ID,
	}
	htmx.OpenDialog(w, "modal")
	return burrow.Render(w, r, http.StatusOK, "notes/form", data)
}

// Update binds, validates, and updates an existing note.
func (a *App) Update(w http.ResponseWriter, r *http.Request) error {
	user := auth.MustCurrentUser[Profile](r.Context())
	id := chi.URLParam(r, "id")

	note, err := a.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return burrow.NewHTTPError(http.StatusNotFound, "note not found")
		}
		return err
	}

	action := "/notes/" + note.ID

	f := forms.FromModel(note, noteFormOpts()...)
	if !f.Bind(r) {
		return burrow.Render(w, r, http.StatusUnprocessableEntity, "notes/form", map[string]any{
			"Fields":         f.Fields(),
			"NonFieldErrors": f.NonFieldErrors(),
			"TitleKey":       "notes-edit-title",
			"Action":         action,
		})
	}

	updated := f.Instance()
	updated.ID = note.ID
	updated.UserID = note.UserID

	if err := a.repo.Update(r.Context(), updated); err != nil {
		return err
	}

	if err := messages.AddSuccess(w, r, i18n.T(r.Context(), "notes-updated")); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to add flash message")
	}

	// HTMX: replace existing card via OOB + close the dialog.
	htmx.CloseDialog(w, "modal")
	return htmx.RenderOrRedirect(w, r, "/notes", "notes/update_response", map[string]any{
		"Note":     updated,
		"Messages": messages.Get(r.Context()),
	})
}

// Delete removes a note owned by the authenticated user.
func (a *App) Delete(w http.ResponseWriter, r *http.Request) error {
	user := auth.MustCurrentUser[Profile](r.Context())
	id := chi.URLParam(r, "id")

	if err := a.repo.Delete(r.Context(), id, user.ID); err != nil {
		return err
	}

	if err := messages.AddSuccess(w, r, i18n.T(r.Context(), "notes-deleted")); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to add flash message")
	}

	remaining, err := a.repo.CountByUserID(r.Context(), user.ID)
	if err != nil {
		return err
	}

	return htmx.RenderOrRedirect(w, r, "/notes", "notes/delete_response", map[string]any{
		"Messages": messages.Get(r.Context()),
		"Empty":    remaining == 0,
	})
}
