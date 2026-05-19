// Package notes is an example custom app demonstrating the burrow framework.
package notes

import (
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/den/document"
	"github.com/urfave/cli/v3"
)

//go:embed translations
var translationFS embed.FS

//go:embed templates
var noteTemplateFS embed.FS

// App implements the notes contrib app.
type App struct {
	repo     *Repository
	userRepo *auth.Repository
}

// New creates a new notes app.
func New() *App {
	return &App{}
}

func (a *App) Name() string { return "notes" }

func (a *App) Dependencies() []string { return []string{"auth"} }

func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
	a.repo = NewRepository(cfg.DB)
	a.userRepo = auth.NewRepository(cfg.DB)
	return nil
}

func (a *App) TranslationFS() fs.FS { return translationFS }

// Documents returns the Den document types registered by this app.
func (a *App) Documents() []document.Document {
	return []document.Document{&Note{}}
}

// TemplateFS returns the embedded HTML template files.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(noteTemplateFS, "templates")
	return sub
}

func (a *App) NavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{
			Label:    "Notes",
			URL:      "/notes",
			Position: 20,
			AuthOnly: true,
		},
	}
}

// AdminRoutes registers admin routes for notes management.
func (a *App) AdminRoutes(r chi.Router) {
	r.Get("/notes", burrow.Handle(a.adminListNotes))
	r.Delete("/notes/{id}", burrow.Handle(a.adminDeleteNote))
}

func (a *App) AdminNavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{
			Label:     "Notes",
			URL:       "/admin/notes",
			Icon:      "notes/icon_journal_text",
			Position:  30,
			AdminOnly: true,
		},
	}
}

func (a *App) Routes(r chi.Router) {
	r.Route("/notes", func(r chi.Router) {
		r.Use(auth.RequireAuth())
		r.Get("/", burrow.Handle(a.List))
		r.Get("/new", burrow.Handle(a.New))
		r.Post("/", burrow.Handle(a.Create))
		r.Get("/{id}", burrow.Handle(a.Edit))
		r.Post("/{id}", burrow.Handle(a.Update))
		r.Delete("/{id}", burrow.Handle(a.Delete))
	})
}

// adminListNotes handles GET /admin/notes — paginated note list with optional search.
func (a *App) adminListNotes(w http.ResponseWriter, r *http.Request) error {
	pr := burrow.ParsePageRequest(r)
	searchTerm := r.URL.Query().Get("q")

	var (
		notes []Note
		page  burrow.PageResult
		err   error
	)
	if searchTerm != "" {
		notes, page, err = a.repo.SearchAllPaged(r.Context(), searchTerm, pr)
	} else {
		notes, page, err = a.repo.ListAllPaged(r.Context(), pr)
	}
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list notes")
	}

	// Resolve usernames for the notes' user IDs in one batch lookup so the
	// admin table can show "alice" instead of "01KQJ6A23JTN2R0WWK5M11G3WG".
	usernames := make(map[string]string)
	if a.userRepo != nil {
		userIDs := make(map[string]struct{}, len(notes))
		for _, n := range notes {
			userIDs[n.UserID] = struct{}{}
		}
		for id := range userIDs {
			u, err := a.userRepo.GetUserByID(r.Context(), id)
			switch {
			case err == nil:
				usernames[id] = u.Username
			case errors.Is(err, auth.ErrNotFound):
				// User was deleted — fall through to ID-only display.
			default:
				slog.Warn("notes admin: username lookup failed", "user_id", id, "error", err)
			}
		}
	}

	return burrow.Render(w, r, http.StatusOK, "notes/admin_list", map[string]any{
		"Notes":      notes,
		"Page":       page,
		"SearchTerm": searchTerm,
		"Usernames":  usernames,
		"RawQuery":   r.URL.RawQuery,
	})
}

// adminDeleteNote handles DELETE /admin/notes/{id} — delete a note.
func (a *App) adminDeleteNote(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	if id == "" {
		return burrow.NewHTTPError(http.StatusBadRequest, "missing note id")
	}

	// Admin delete — no user ownership check.
	if err := a.repo.DeleteByID(r.Context(), id); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to delete note")
	}

	htmx.SmartRedirect(w, r, "/admin/notes")
	return nil
}
