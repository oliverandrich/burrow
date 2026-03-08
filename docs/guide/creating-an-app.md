# Creating an App

This guide walks through building a custom app from scratch, using a "notes" app as the example.

## The App Interface

Every app implements `burrow.App`:

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
- JSON tags control API serialisation

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

## Step 4: Write the Templates

Create `templates/list.html`:

```html
{{ define "notes/list" -}}
<h1>{{ t "notes-title" }}</h1>
<ul>
  {{ range .Notes }}
    <li><a href="/notes/{{ .ID }}">{{ .Title }}</a></li>
  {{ end }}
</ul>
{{- end }}
```

Embed the templates:

```go
//go:embed templates/*.html
var templateFS embed.FS
```

## Step 5: Write the Handlers

```go
type Handlers struct {
    repo *Repository
}

func NewHandlers(repo *Repository) *Handlers {
    return &Handlers{repo: repo}
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
    user := auth.UserFromContext(r.Context())
    if user == nil {
        return burrow.NewHTTPError(http.StatusUnauthorized, "not authenticated")
    }

    notes, err := h.repo.ListByUserID(r.Context(), user.ID)
    if err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list notes")
    }

    return burrow.RenderTemplate(w, r, http.StatusOK, "notes/list", map[string]any{
        "Title": "My Notes",
        "Notes": notes,
    })
}

func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) error {
    user := auth.UserFromContext(r.Context())
    if user == nil {
        return burrow.NewHTTPError(http.StatusUnauthorized, "not authenticated")
    }

    var req struct {
        Title   string `form:"title"   validate:"required"`
        Content string `form:"content"`
    }
    if err := burrow.Bind(r, &req); err != nil {
        return err // (1)!
    }

    note := &Note{
        UserID:  user.ID,
        Title:   req.Title,
        Content: req.Content,
    }

    if err := h.repo.Create(r.Context(), note); err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to create note")
    }

    http.Redirect(w, r, "/notes", http.StatusSeeOther)
    return nil
}
```

1. `Bind` decodes the request body **and** validates it. Returns a `*burrow.ValidationError` when validation fails — see [Validation](validation.md).

!!! note "How `Handle()` processes errors"
    `burrow.Handle()` wraps your `HandlerFunc` and converts returned errors to HTTP responses:

    - **`*HTTPError`** — sends the error's status code and message as a plain text response (e.g., `NewHTTPError(404, "not found")` sends a 404)
    - **`*ValidationError`** — not handled automatically; your handler must check for it and re-render the form
    - **Other errors** — sends a generic 500 Internal Server Error (the original error is logged but not exposed to the client)

## Step 6: Assemble the App

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

func (a *App) Register(cfg *burrow.AppConfig) error {
    a.repo = NewRepository(cfg.DB)
    a.handlers = NewHandlers(a.repo)
    return nil
}

func (a *App) MigrationFS() fs.FS { // (2)!
    sub, _ := fs.Sub(migrationFS, "migrations")
    return sub
}

func (a *App) TemplateFS() fs.FS { // (3)!
    sub, _ := fs.Sub(templateFS, "templates")
    return sub
}

func (a *App) NavItems() []burrow.NavItem { // (4)!
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

func (a *App) Routes(r chi.Router) { // (5)!
    r.Route("/notes", func(r chi.Router) {
        r.Use(auth.RequireAuth())
        r.Get("/", burrow.Handle(a.handlers.List))
        r.Post("/", burrow.Handle(a.handlers.Create))
    })
}
```

1. `HasDependencies` — ensures `auth` is registered before this app
2. `Migratable` — the framework runs SQL migrations at startup
3. `HasTemplates` — contributes `.html` template files to the global template set
4. `HasNavItems` — contributes navigation entries to layouts
5. `HasRoutes` — registers HTTP handlers on the Chi router

## File Layout

For multi-file apps, name files by their purpose rather than repeating the package name:

| File | Content |
|------|---------|
| `app.go` | App struct, `Name()`, `Register()`, `Routes()`, framework wiring |
| `context.go` | Package doc comment, context key types, context helpers |
| `handlers.go` | HTTP handlers |
| `middleware.go` | Middleware functions |
| `models.go` | Domain models |
| `repository.go` | Data access layer |
| `templates/` | HTML template files (`{{ define "appname/..." }}`) |

Small apps can keep everything in `app.go` — split only when a file grows large or mixes distinct responsibilities.

## Step 7: Register the App

In `main.go`:

```go
srv := burrow.NewServer(
    session.New(),
    auth.New(),
    healthcheck.New(),
    notes.New(), // Add your app here
)
```

!!! info "Auto-sorting"
    `NewServer` automatically sorts apps by their `HasDependencies` declarations. You can list them in any order, and the framework will ensure dependencies are registered first.

## Optional Interfaces

Your app can implement any combination of these interfaces:

| Interface | Method | Purpose |
|-----------|--------|---------|
| `Migratable` | `MigrationFS() fs.FS` | Provide SQL migrations |
| `HasRoutes` | `Routes(r chi.Router)` | Register HTTP handlers |
| `HasMiddleware` | `Middleware() []func(http.Handler) http.Handler` | Add global middleware |
| `HasNavItems` | `NavItems() []burrow.NavItem` | Contribute navigation entries |
| `HasTemplates` | `TemplateFS() fs.FS` | Contribute HTML template files |
| `HasFuncMap` | `FuncMap() template.FuncMap` | Contribute static template functions |
| `HasRequestFuncMap` | `RequestFuncMap(r *http.Request) template.FuncMap` | Contribute request-scoped template functions |
| `Configurable` | `Flags() []cli.Flag` + `Configure(cmd *cli.Command) error` | Add CLI flags |
| `HasCLICommands` | `CLICommands() []*cli.Command` | Add CLI subcommands |
| `Seedable` | `Seed(ctx context.Context) error` | Seed initial data |
| `HasDependencies` | `Dependencies() []string` | Declare required apps |
| `HasAdmin` | `AdminRoutes(r chi.Router)` + `AdminNavItems() []NavItem` | Contribute admin panel |
| `HasStaticFiles` | `StaticFS() (prefix string, fsys fs.FS)` | Contribute static assets |
| `HasTranslations` | `TranslationFS() fs.FS` | Contribute translation files |
| `HasShutdown` | `Shutdown(ctx context.Context) error` | Clean up on shutdown |

See [Core Interfaces](../reference/interfaces.md) for the full reference.
