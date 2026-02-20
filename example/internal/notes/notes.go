// Package notes is an example custom app demonstrating the webstack framework.
package notes

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/oliverandrich/go-webapp-template/contrib/auth"
	"codeberg.org/oliverandrich/go-webapp-template/core"
	"github.com/labstack/echo/v5"
	"github.com/uptrace/bun"
)

//go:embed migrations
var migrationFS embed.FS

// Note represents a user's note.
type Note struct { //nolint:govet // fieldalignment: readability over optimization
	bun.BaseModel `bun:"table:notes,alias:n"`

	ID        int64     `bun:",pk,autoincrement" json:"id"`
	UserID    int64     `bun:",notnull" json:"user_id"`
	Title     string    `bun:",notnull" json:"title"`
	Content   string    `bun:",notnull,default:''" json:"content"`
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
	DeletedAt time.Time `bun:",soft_delete,nullzero" json:"-"`
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
	repo     *Repository
	handlers *Handlers
}

// New creates a new notes app.
func New() *App {
	return &App{}
}

func (a *App) Name() string { return "notes" }

func (a *App) Dependencies() []string { return []string{"auth"} }

func (a *App) Register(cfg *core.AppConfig) error {
	a.repo = NewRepository(cfg.DB)
	a.handlers = NewHandlers(a.repo)
	return nil
}

func (a *App) MigrationFS() fs.FS {
	sub, _ := fs.Sub(migrationFS, "migrations")
	return sub
}

func (a *App) NavItems() []core.NavItem {
	return []core.NavItem{
		{
			Label:    "Notes",
			URL:      "/notes",
			Icon:     "bi bi-journal-text",
			Position: 20,
			AuthOnly: true,
		},
	}
}

func (a *App) Routes(e *echo.Echo) {
	if a.handlers == nil {
		return
	}
	h := a.handlers

	g := e.Group("/notes", auth.RequireAuth())
	g.GET("", h.List)
	g.POST("", h.Create)
	g.DELETE("/:id", h.Delete)
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

// List renders the user's notes as JSON.
func (h *Handlers) List(c *echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	notes, err := h.repo.ListByUserID(c.Request().Context(), user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list notes")
	}

	return c.JSON(http.StatusOK, notes)
}

// Create adds a new note for the authenticated user.
func (h *Handlers) Create(c *echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	title := c.FormValue("title")
	if title == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "title is required")
	}

	note := &Note{
		UserID:  user.ID,
		Title:   title,
		Content: c.FormValue("content"),
	}

	if err := h.repo.Create(c.Request().Context(), note); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create note")
	}

	return c.Redirect(http.StatusSeeOther, "/notes")
}

// Delete removes a note owned by the authenticated user.
func (h *Handlers) Delete(c *echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid note id")
	}

	if err := h.repo.Delete(c.Request().Context(), id, user.ID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete note")
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}
