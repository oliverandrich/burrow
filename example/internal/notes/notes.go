// Package notes is an example custom app demonstrating the burrow framework.
package notes

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/admin/modeladmin"
	matpl "codeberg.org/oliverandrich/burrow/contrib/admin/modeladmin/templates"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"codeberg.org/oliverandrich/burrow/contrib/bsicons"
	"codeberg.org/oliverandrich/burrow/contrib/messages"
	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"
)

//go:embed migrations
var migrationFS embed.FS

//go:embed translations
var translationFS embed.FS

//go:embed templates
var noteTemplateFS embed.FS

// Note represents a user's note.
type Note struct { //nolint:govet // fieldalignment: readability over optimization
	bun.BaseModel `bun:"table:notes,alias:n"`

	ID        int64     `bun:",pk,autoincrement" json:"id"`
	UserID    int64     `bun:",notnull" json:"user_id" form:"-"`
	Title     string    `bun:",notnull" json:"title" form:"label=Title"`
	Content   string    `bun:",notnull,default:''" json:"content" form:"label=Content,widget=textarea"`
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at" form:"-"`
	DeletedAt time.Time `bun:",soft_delete,nullzero" json:"-" form:"-"`
}

// Repository provides data access for notes.
type Repository struct {
	db *bun.DB
}

// NewRepository creates a new notes repository.
func NewRepository(db *bun.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new note.
func (r *Repository) Create(ctx context.Context, note *Note) error {
	if _, err := r.db.NewInsert().Model(note).Exec(ctx); err != nil {
		return fmt.Errorf("create note: %w", err)
	}
	return nil
}

// ListByUserID returns all notes for a user, most recent first.
func (r *Repository) ListByUserID(ctx context.Context, userID int64) ([]Note, error) {
	var notes []Note
	if err := r.db.NewSelect().Model(&notes).
		Where("user_id = ?", userID).
		Order("created_at DESC", "id DESC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list notes for user %d: %w", userID, err)
	}
	return notes, nil
}

// ListByUserIDPaged returns paginated notes for a user using cursor-based pagination.
// Notes are ordered by ID descending (newest first).
func (r *Repository) ListByUserIDPaged(ctx context.Context, userID int64, pr burrow.PageRequest) ([]Note, burrow.PageResult, error) {
	var notes []Note
	q := r.db.NewSelect().Model(&notes).Where("user_id = ?", userID)
	q = burrow.ApplyCursor(q, pr, "id")
	if err := q.Scan(ctx); err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("list notes for user %d: %w", userID, err)
	}

	notes, hasMore := burrow.TrimCursorResults(notes, pr.Limit)
	var lastID string
	if len(notes) > 0 {
		lastID = strconv.FormatInt(notes[len(notes)-1].ID, 10)
	}

	return notes, burrow.CursorResult(lastID, hasMore), nil
}

// Delete soft-deletes a note owned by the given user.
func (r *Repository) Delete(ctx context.Context, noteID, userID int64) error {
	if _, err := r.db.NewDelete().Model((*Note)(nil)).
		Where("id = ? AND user_id = ?", noteID, userID).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete note %d: %w", noteID, err)
	}
	return nil
}

// --- App ---

// App implements the notes contrib app.
type App struct {
	repo       *Repository
	handlers   *Handlers
	notesAdmin *modeladmin.ModelAdmin[Note]
}

// New creates a new notes app.
func New() *App {
	return &App{}
}

func (a *App) Name() string { return "notes" }

func (a *App) Dependencies() []string { return []string{"auth"} }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.repo = NewRepository(cfg.DB)
	a.handlers = NewHandlers(a.repo)
	a.notesAdmin = &modeladmin.ModelAdmin[Note]{
		Slug:       "notes",
		Display:    "Notes",
		DB:         cfg.DB,
		Renderer:   matpl.DefaultRenderer[Note](),
		CanCreate:  true,
		CanEdit:    true,
		CanDelete:  true,
		ListFields: []string{"ID", "Title", "Content", "UserID", "CreatedAt"},
		OrderBy:    "created_at DESC, id DESC",
	}
	return nil
}

func (a *App) TranslationFS() fs.FS { return translationFS }

func (a *App) MigrationFS() fs.FS {
	sub, _ := fs.Sub(migrationFS, "migrations")
	return sub
}

// TemplateFS returns the embedded HTML template files.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(noteTemplateFS, "templates")
	return sub
}

// FuncMap returns template functions used by notes templates.
func (a *App) FuncMap() template.FuncMap {
	return template.FuncMap{
		"iconPlusLg":      func(class ...string) template.HTML { return bsicons.PlusLg(class...) },
		"iconJournalText": func(class ...string) template.HTML { return bsicons.JournalText(class...) },
	}
}

func (a *App) NavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{
			Label:    "Notes",
			URL:      "/notes",
			Icon:     bsicons.JournalText(),
			Position: 20,
			AuthOnly: true,
		},
	}
}

func (a *App) AdminRoutes(r chi.Router) {
	if a.notesAdmin == nil {
		return
	}
	a.notesAdmin.Routes(r)
}

func (a *App) AdminNavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{
			Label:     "Notes",
			LabelKey:  "admin-nav-notes",
			URL:       "/admin/notes",
			Icon:      bsicons.JournalText(),
			Position:  30,
			AdminOnly: true,
		},
	}
}

func (a *App) Routes(r chi.Router) {
	if a.handlers == nil {
		return
	}
	h := a.handlers

	r.Route("/notes", func(r chi.Router) {
		r.Use(auth.RequireAuth())
		r.Get("/", burrow.Handle(h.List))
		r.Post("/", burrow.Handle(h.Create))
		r.Delete("/{id}", burrow.Handle(h.Delete))
	})
}

// --- Handlers ---

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
	notes, page, err := h.repo.ListByUserIDPaged(r.Context(), user.ID, pr)
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list notes")
	}

	exec := burrow.TemplateExecutorFromContext(r.Context())
	if exec == nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "no template executor")
	}

	data := map[string]any{
		"Notes": notes,
		"Page":  page,
		"Title": "Notes",
	}

	// HTMX infinite scroll: return only the notes fragment.
	if r.Header.Get("HX-Request") == "true" && pr.Cursor != "" {
		content, execErr := exec(r, "notes/notes_page", data)
		if execErr != nil {
			return execErr
		}
		return burrow.Render(w, r, http.StatusOK, content)
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
	if r.Header.Get("HX-Request") == "true" {
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
