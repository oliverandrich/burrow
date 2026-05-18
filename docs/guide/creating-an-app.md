# Creating an App

This guide walks through building a custom app from scratch, using a "notes" app as the example.

!!! tip "Skip the boilerplate with `burrow generate app`"
    For a minimal stub with `app.go`, `app_test.go`, and a starter
    template, run:

    ```bash
    burrow generate app notes
    ```

    Output lands at `./internal/notes/` (override with `--path`). The
    stub gives you the App struct, a registered route, and an
    embedded template — you fill in the model, repository, and
    handlers below. See [CLI reference](../reference/cli.md#burrow-generate-app)
    for all flags.

## The App Interface

Every app implements `burrow.App`:

```go
type App interface {
    Name() string
}
```

`Name()` returns a unique identifier. Apps that need setup (database access, flag values) implement `Configurable` — see [Configuration](configuration.md).

## Step 1: Define the Model

`models.go`:

```go
package notes

import (
    "time"
    "github.com/oliverandrich/den/document"
)

type Note struct {
    document.Base
    UserID    string    `json:"user_id"`
    Title     string    `json:"title" den:"index"`
    Content   string    `json:"content"`
    CreatedAt time.Time `json:"created_at"`
}
```

Key points:

- `document.Base` provides ULID-based ID, revision, and timestamps
- `json` tags control serialization, `den` tags add indexes

## Step 2: Register Documents

Apps declare their document types by implementing `HasDocuments`. Den creates tables and indexes automatically on startup — no SQL migration files needed.

In `app.go`:

```go
//go:embed templates/*.html
var templateFS embed.FS

//go:embed translations
var translationFS embed.FS

type App struct {
    repo *Repository
}

func New() *App { return &App{} }

func (a *App) Name() string { return "notes" }

// Documents tells Burrow which document types this app uses.
// Tables and indexes are created automatically on startup.
func (a *App) Documents() []document.Document {
    return []document.Document{&Note{}}
}
```

## Step 3: Create the Repository

`repository.go`:

```go
type Repository struct {
    db *den.DB
}

func NewRepository(db *den.DB) *Repository {
    return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, note *Note) error {
    return den.Save(ctx, r.db, note)
}

func (r *Repository) ListByUserID(ctx context.Context, userID string) ([]Note, error) {
    return den.NewQuery[Note](r.db,
        where.Field("user_id").Eq(userID),
    ).Sort("created_at", den.Desc).All(ctx)
}

func (r *Repository) Delete(ctx context.Context, noteID, userID string) error {
    note, err := den.NewQuery[Note](r.db,
        where.Field("id").Eq(noteID),
        where.Field("user_id").Eq(userID),
    ).First(ctx)
    if err != nil {
        return err
    }
    return den.Delete(ctx, r.db, note)
}
```

## Step 4: Write the Templates

`templates/list.html`:

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

The `{{ t "notes-title" }}` call uses the `t` template function provided by the [i18n system](i18n.md). It looks up a translation key from your app's TOML files. If you don't need i18n, use a plain string instead: `<h1>My Notes</h1>`.

## Step 5: Write the Handlers

`handlers.go`:

```go
import (
    "net/http"

    "github.com/oliverandrich/burrow"
    "github.com/oliverandrich/burrow/contrib/auth"
)

func (a *App) List(w http.ResponseWriter, r *http.Request) error {
    user := auth.CurrentUser(r.Context())
    if user == nil {
        return burrow.NewHTTPError(http.StatusUnauthorized, "not authenticated")
    }

    notes, err := a.repo.ListByUserID(r.Context(), user.ID)
    if err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list notes")
    }

    return burrow.Render(w, r, http.StatusOK, "notes/list", map[string]any{
        "Notes": notes,
    })
}

func (a *App) Create(w http.ResponseWriter, r *http.Request) error {
    user := auth.CurrentUser(r.Context())
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

    if err := a.repo.Create(r.Context(), note); err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to create note")
    }

    http.Redirect(w, r, "/notes", http.StatusSeeOther)
    return nil
}
```

1. `Bind` decodes the request body **and** validates it. Returns a `*burrow.ValidationError` when validation fails — see [Validation](validation.md).

!!! tip "Handlers are methods on `*App`"
    Handlers are defined as methods on your `*App` struct, giving them direct access to all dependencies (repositories, services, config) without extra wiring. No separate `Handlers` struct needed.

!!! note "How `Handle()` processes errors"
    See the [Routing guide](routing.md#error-handling) for details on how `burrow.Handle()` converts returned errors to HTTP responses.

## Step 6: Assemble the App

`app.go`:

```go
type App struct {
    repo *Repository
}

func New() *App {
    return &App{}
}

func (a *App) Name() string { return "notes" }

func (a *App) Dependencies() []string { return []string{"auth"} } // (1)!

func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
    a.repo = NewRepository(cfg.DB)
    return nil
}

func (a *App) Documents() []document.Document { // (2)!
    return []document.Document{&Note{}}
}

func (a *App) TemplateFS() fs.FS { // (3)!
    sub, _ := fs.Sub(templateFS, "templates")
    return sub
}

func (a *App) TranslationFS() fs.FS { return translationFS } // (6)!

func (a *App) NavItems() []burrow.NavItem { // (4)!
    return []burrow.NavItem{
        {
            Label:    "Notes",
            URL:      "/notes",
            Icon:     "notes/icon_journal_text", // see Navigation › Icon convention
            Position: 20,
            AuthOnly: true,
        },
    }
}

func (a *App) Routes(r chi.Router) { // (5)!
    r.Route("/notes", func(r chi.Router) {
        r.Use(auth.RequireAuth())
        r.Get("/", burrow.Handle(a.List))
        r.Post("/", burrow.Handle(a.Create))
    })
}
```

1. `HasDependencies` — ensures `auth` is registered before this app
2. `HasDocuments` — the framework registers document types and creates collections at startup
3. `HasTemplates` — contributes `.html` template files to the global template set. Uses `fs.Sub()` to strip the `templates/` prefix.
4. `HasNavItems` — contributes navigation entries to layouts. Entries with `AuthOnly: true` are only shown to authenticated users — this filtering is handled automatically by the layout.
5. `HasRoutes` — registers HTTP handlers on the Chi router
6. `HasTranslations` — contributes TOML translation files. Returns the `embed.FS` directly (not `fs.Sub`) because the i18n loader expects the `translations/` directory to be present.

## File Layout

For multi-file apps, name files by their purpose rather than repeating the package name:

| File | Content |
|------|---------|
| `app.go` | App struct, `Name()`, `Configure()`, `Routes()`, framework wiring |
| `context.go` | Package doc comment, context key types, context helpers |
| `handlers.go` | HTTP handlers |
| `middleware.go` | Middleware functions |
| `models.go` | Domain models |
| `repository.go` | Data access layer |
| `templates/` | HTML template files (`{{ define "appname/..." }}`) |

Small apps can keep everything in `app.go` — split only when a file grows large or mixes distinct responsibilities.

## Step 7: Register the App

`cmd/server/main.go`:

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
| `HasDocuments` | `Documents() []document.Document` | Register document types |
| `HasRoutes` | `Routes(r chi.Router)` | Register HTTP handlers |
| `HasMiddleware` | `Middleware() []func(http.Handler) http.Handler` | Add global middleware |
| `HasNavItems` | `NavItems() []burrow.NavItem` | Contribute navigation entries |
| `HasTemplates` | `TemplateFS() fs.FS` | Contribute HTML template files |
| `HasFuncMap` | `FuncMap() template.FuncMap` | Contribute static template functions |
| `HasRequestFuncMap` | `RequestFuncMap(ctx context.Context) template.FuncMap` | Contribute context-scoped template functions |
| `HasFlags` | `Flags(configSource func(key string) cli.ValueSource) []cli.Flag` | Add CLI flags |
| `Configurable` | `Configure(cfg *AppConfig, cmd *cli.Command) error` | App initialisation and configuration |
| `HasCLICommands` | `CLICommands() []*cli.Command` | Add CLI subcommands |
| `Seedable` | `Seed(ctx context.Context) error` | Seed initial data |
| `HasDependencies` | `Dependencies() []string` | Declare required apps |
| `HasAdmin` | `AdminRoutes(r chi.Router)` + `AdminNavItems() []NavItem` | Contribute admin panel |
| `HasStaticFiles` | `StaticFS() (prefix string, fsys fs.FS)` | Contribute static assets |
| `HasTranslations` | `TranslationFS() fs.FS` | Contribute translation files |
| `HasJobs` | `RegisterJobs(q Queue)` | Register background job handlers |
| `PostConfigurable` | `PostConfigure(cfg *AppConfig, cmd *cli.Command) error` | Second-pass configuration after all apps are configured |
| `HasShutdown` | `Shutdown(ctx context.Context) error` | Clean up on shutdown |

See [Core Interfaces](../reference/interfaces.md) for the full reference.

### Example: Writing Middleware

Middleware follows the standard Go pattern — `func(http.Handler) http.Handler`. Return it from `Middleware()` to apply it globally:

```go
// Middleware adds a request timing header to all responses.
func (a *App) Middleware() []func(http.Handler) http.Handler {
    return []func(http.Handler) http.Handler{
        func(next http.Handler) http.Handler {
            return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                start := time.Now()
                next.ServeHTTP(w, r)
                w.Header().Set("X-Response-Time", time.Since(start).String())
            })
        },
    }
}
```

Middleware from all apps runs in dependency-sorted order — apps registered earlier in `NewServer()` that have no dependency conflicts have their middleware applied first. Within a single app, middleware runs in the slice order returned by `Middleware()`.
