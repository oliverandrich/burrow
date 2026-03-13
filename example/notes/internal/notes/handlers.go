package notes

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/burrow/forms"
	"github.com/oliverandrich/burrow/i18n"
)

// noteFormOpts returns the common form options for the Note form.
func noteFormOpts() []forms.Option[Note] {
	return []forms.Option[Note]{
		forms.WithExclude[Note]("ID", "UserID", "CreatedAt"),
	}
}

// Handlers holds the notes HTTP handlers.
type Handlers struct {
	repo *Repository
}

// NewHandlers creates notes handlers.
func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

// List renders the user's notes as an HTML page with cursor-based pagination.
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		return burrow.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	pr := burrow.ParsePageRequest(r)
	searchQuery := r.URL.Query().Get("q")

	var notes []Note
	var page burrow.PageResult
	var err error

	if searchQuery != "" {
		notes, page, err = h.repo.SearchByUserID(r.Context(), user.ID, searchQuery, pr)
	} else {
		notes, page, err = h.repo.ListByUserIDPaged(r.Context(), user.ID, pr)
	}
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list notes")
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
		case pr.Cursor != "":
			tmpl = "notes/notes_page"
		case r.URL.Query().Has("q"):
			tmpl = "notes/notes_list"
		}
	}

	return burrow.RenderTemplate(w, r, http.StatusOK, tmpl, data)
}

// New renders the empty create form.
// HTMX: returns the form fragment for inline insertion.
// Non-HTMX: returns the form wrapped in the layout.
func (h *Handlers) New(w http.ResponseWriter, r *http.Request) error {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		return burrow.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	f := forms.New[Note](noteFormOpts()...)
	data := map[string]any{
		"Fields":   f.Fields(),
		"TitleKey": "notes-new-title",
		"Action":   "/notes",
	}
	return burrow.RenderTemplate(w, r, http.StatusOK, "notes/form", data)
}

// Create adds a new note for the authenticated user.
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) error {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		return burrow.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	f := forms.New[Note](noteFormOpts()...)
	if !f.Bind(r) {
		return burrow.RenderTemplate(w, r, http.StatusUnprocessableEntity, "notes/form", map[string]any{
			"Fields":         f.Fields(),
			"NonFieldErrors": f.NonFieldErrors(),
			"TitleKey":       "notes-new-title",
			"Action":         "/notes",
		})
	}

	note := f.Instance()
	note.UserID = user.ID

	if err := h.repo.Create(r.Context(), note); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to create note")
	}

	if err := messages.AddSuccess(w, r, i18n.T(r.Context(), "notes-created")); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to add flash message")
	}

	// HTMX: prepend new card via OOB + clear the form.
	if htmx.Request(r).IsHTMX() {
		return burrow.RenderTemplate(w, r, http.StatusOK, "notes/create_response", map[string]any{
			"Note":     note,
			"Messages": messages.Get(r.Context()),
		})
	}

	http.Redirect(w, r, "/notes", http.StatusSeeOther)
	return nil
}

// Edit renders the edit form pre-filled with an existing note.
func (h *Handlers) Edit(w http.ResponseWriter, r *http.Request) error {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		return burrow.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid note id")
	}

	note, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		return burrow.NewHTTPError(http.StatusNotFound, "note not found")
	}

	f := forms.FromModel(note, noteFormOpts()...)
	data := map[string]any{
		"Fields":   f.Fields(),
		"TitleKey": "notes-edit-title",
		"Action":   "/notes/" + strconv.FormatInt(note.ID, 10),
	}
	return burrow.RenderTemplate(w, r, http.StatusOK, "notes/form", data)
}

// Update binds, validates, and updates an existing note.
func (h *Handlers) Update(w http.ResponseWriter, r *http.Request) error {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		return burrow.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid note id")
	}

	note, err := h.repo.GetByID(r.Context(), id, user.ID)
	if err != nil {
		return burrow.NewHTTPError(http.StatusNotFound, "note not found")
	}

	action := "/notes/" + strconv.FormatInt(note.ID, 10)

	f := forms.FromModel(note, noteFormOpts()...)
	if !f.Bind(r) {
		return burrow.RenderTemplate(w, r, http.StatusUnprocessableEntity, "notes/form", map[string]any{
			"Fields":         f.Fields(),
			"NonFieldErrors": f.NonFieldErrors(),
			"TitleKey":       "notes-edit-title",
			"Action":         action,
		})
	}

	updated := f.Instance()
	updated.ID = note.ID
	updated.UserID = note.UserID

	if err := h.repo.Update(r.Context(), updated); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to update note")
	}

	if err := messages.AddSuccess(w, r, i18n.T(r.Context(), "notes-updated")); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to add flash message")
	}

	// HTMX: replace existing card via OOB + clear the form.
	if htmx.Request(r).IsHTMX() {
		return burrow.RenderTemplate(w, r, http.StatusOK, "notes/update_response", map[string]any{
			"Note":     updated,
			"Messages": messages.Get(r.Context()),
		})
	}

	http.Redirect(w, r, "/notes", http.StatusSeeOther)
	return nil
}

// Delete removes a note owned by the authenticated user.
func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) error {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		return burrow.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid note id")
	}

	if err := h.repo.Delete(r.Context(), id, user.ID); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to delete note")
	}

	if err := messages.AddSuccess(w, r, i18n.T(r.Context(), "notes-deleted")); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to add flash message")
	}

	return burrow.RenderTemplate(w, r, http.StatusOK, "app/alerts_oob", map[string]any{
		"Messages": messages.Get(r.Context()),
	})
}
