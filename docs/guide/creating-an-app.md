# Creating an App

This guide walks through building a custom app from scratch, using a "notes" app as the example.

## The App Interface

Every app implements `core.App`:

```go
type App interface {
    Name() string
    Register(cfg *AppConfig) error
}
```

`Name()` returns a unique identifier. `Register()` receives the shared `AppConfig` with the database, registry, config, and layouts.

## Step 1: Define the Model

```go
package notes

import (
    "time"
    "github.com/uptrace/bun"
)

type Note struct {
    bun.BaseModel `bun:"table:notes,alias:n"`

    ID        int64     `bun:",pk,autoincrement" json:"id"`
    UserID    int64     `bun:",notnull" json:"user_id"`
    Title     string    `bun:",notnull" json:"title"`
    Content   string    `bun:",notnull,default:''" json:"content"`
    CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
    DeletedAt time.Time `bun:",soft_delete,nullzero" json:"-"`
}
```

Key points:

- `bun.BaseModel` with table name and alias
- `bun:",soft_delete,nullzero"` on `DeletedAt` enables soft-delete
- JSON tags control API serialization

## Step 2: Write the Migration

Create `migrations/001_create_notes.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_notes_user_id ON notes (user_id);
```

Embed it in the package:

```go
import "embed"

//go:embed migrations
var migrationFS embed.FS
```

## Step 3: Create the Repository

```go
type Repository struct {
    db *bun.DB
}

func NewRepository(db *bun.DB) *Repository {
    return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, note *Note) error {
    _, err := r.db.NewInsert().Model(note).Exec(ctx)
    return err
}

func (r *Repository) ListByUserID(ctx context.Context, userID int64) ([]Note, error) {
    var notes []Note
    err := r.db.NewSelect().Model(&notes).
        Where("user_id = ?", userID).
        Order("created_at DESC").
        Scan(ctx)
    return notes, err
}

func (r *Repository) Delete(ctx context.Context, noteID, userID int64) error {
    _, err := r.db.NewDelete().Model((*Note)(nil)).
        Where("id = ? AND user_id = ?", noteID, userID).
        Exec(ctx)
    return err
}
```

## Step 4: Write the Handlers

```go
type Handlers struct {
    repo *Repository
}

func NewHandlers(repo *Repository) *Handlers {
    return &Handlers{repo: repo}
}

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

func (h *Handlers) Create(c *echo.Context) error {
    user := auth.GetUser(c)
    if user == nil {
        return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
    }

    note := &Note{
        UserID:  user.ID,
        Title:   c.FormValue("title"),
        Content: c.FormValue("content"),
    }

    if err := h.repo.Create(c.Request().Context(), note); err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, "failed to create note")
    }

    return c.Redirect(http.StatusSeeOther, "/notes")
}
```

## Step 5: Assemble the App

```go
type App struct {
    repo     *Repository
    handlers *Handlers
}

func New() *App {
    return &App{}
}

func (a *App) Name() string { return "notes" }

func (a *App) Dependencies() []string { return []string{"auth"} } // (1)!

func (a *App) Register(cfg *core.AppConfig) error {
    a.repo = NewRepository(cfg.DB)
    a.handlers = NewHandlers(a.repo)
    return nil
}

func (a *App) MigrationFS() fs.FS { // (2)!
    sub, _ := fs.Sub(migrationFS, "migrations")
    return sub
}

func (a *App) NavItems() []core.NavItem { // (3)!
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

func (a *App) Routes(e *echo.Echo) { // (4)!
    g := e.Group("/notes", auth.RequireAuth())
    g.GET("", a.handlers.List)
    g.POST("", a.handlers.Create)
}
```

1. `HasDependencies` — ensures `auth` is registered before this app
2. `Migratable` — the framework runs SQL migrations at startup
3. `HasNavItems` — contributes navigation entries to layouts
4. `HasRoutes` — registers HTTP handlers on the Echo router

## Step 6: Register the App

In `main.go`:

```go
srv := core.NewServer(
    &session.App{},
    auth.New(nil),
    &healthcheck.App{},
    notes.New(), // Add your app here
)
```

!!! warning "Registration Order Matters"
    Apps are registered in the order you pass them to `NewServer`. If your app declares dependencies via `HasDependencies`, the required apps must appear earlier in the list.

## Optional Interfaces

Your app can implement any combination of these interfaces:

| Interface | Method | Purpose |
|-----------|--------|---------|
| `Migratable` | `MigrationFS() fs.FS` | Provide SQL migrations |
| `HasRoutes` | `Routes(e *echo.Echo)` | Register HTTP handlers |
| `HasMiddleware` | `Middleware() []echo.MiddlewareFunc` | Add global middleware |
| `HasNavItems` | `NavItems() []core.NavItem` | Contribute navigation entries |
| `Configurable` | `Flags() []cli.Flag` + `Configure(cmd *cli.Command) error` | Add CLI flags |
| `HasCLICommands` | `CLICommands() []*cli.Command` | Add CLI subcommands |
| `Seedable` | `Seed(ctx context.Context) error` | Seed initial data |
| `HasDependencies` | `Dependencies() []string` | Declare required apps |

See [Core Interfaces](../reference/interfaces.md) for the full reference.
