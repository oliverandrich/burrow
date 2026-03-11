package notes

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/messages"
)

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

	exec := burrow.TemplateExecutorFromContext(r.Context())
	if exec == nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "no template executor")
	}

	data := map[string]any{
		"Notes":       notes,
		"Page":        page,
		"Title":       "Notes",
		"SearchQuery": searchQuery,
	}

	if htmx.Request(r).IsHTMX() {
		tmpl := ""
		switch {
		case pr.Cursor != "":
			// Infinite scroll: return only new cards + next scroll trigger.
			tmpl = "notes/notes_page"
		case r.URL.Query().Has("q"):
			// Search (including empty clear): replace the entire notes grid.
			tmpl = "notes/notes_list"
		}
		if tmpl != "" {
			content, execErr := exec(r, tmpl, data)
			if execErr != nil {
				return execErr
			}
			return burrow.Render(w, r, http.StatusOK, content)
		}
	}

	// Normal + HTMX nav: RenderTemplate handles layout/fragment automatically.
	return burrow.RenderTemplate(w, r, http.StatusOK, "notes/list_page", data)
}

// Create adds a new note for the authenticated user.
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) error {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		return burrow.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	title := r.FormValue("title") //nolint:gosec // G120: body size limited by server-level RequestSize middleware
	if title == "" {
		return burrow.NewHTTPError(http.StatusBadRequest, "title is required")
	}

	note := &Note{
		UserID:  user.ID,
		Title:   title,
		Content: r.FormValue("content"), //nolint:gosec // G120: body size limited by server-level RequestSize middleware
	}

	if err := h.repo.Create(r.Context(), note); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to create note")
	}

	if err := messages.AddSuccess(w, r, "Note created."); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to add flash message")
	}

	// HTMX: return only the new card (prepended via hx-swap="afterbegin") with OOB alerts.
	if htmx.Request(r).IsHTMX() {
		exec := burrow.TemplateExecutorFromContext(r.Context())
		if exec == nil {
			return burrow.NewHTTPError(http.StatusInternalServerError, "no template executor")
		}

		data := map[string]any{
			"Note":     note,
			"Messages": messages.Get(r.Context()),
		}

		content, err := exec(r, "notes/create_response", data)
		if err != nil {
			return err
		}
		return burrow.Render(w, r, http.StatusOK, content)
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

	if err := messages.AddSuccess(w, r, "Note deleted."); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to add flash message")
	}

	exec := burrow.TemplateExecutorFromContext(r.Context())
	if exec == nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "no template executor")
	}

	data := map[string]any{
		"Messages": messages.Get(r.Context()),
	}

	content, execErr := exec(r, "app/alerts_oob", data)
	if execErr != nil {
		return execErr
	}
	return burrow.Render(w, r, http.StatusOK, content)
}
