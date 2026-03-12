# Core Interfaces

All interfaces are defined in the `burrow` package (`github.com/oliverandrich/burrow`).

## Required

### App

Every app must implement this interface:

```go
type App interface {
    Name() string
    Register(cfg *AppConfig) error
}
```

- `Name()` returns a unique identifier for the app (e.g., `"auth"`, `"notes"`)
- `Register()` receives the shared `AppConfig` for initialisation

```go
type myApp struct {
    repo *Repository
}

func (a *myApp) Name() string { return "notes" }

func (a *myApp) Register(cfg *burrow.AppConfig) error {
    a.repo = NewRepository(cfg.DB)
    return nil
}
```

### AppConfig

Passed to every app's `Register` method:

```go
type AppConfig struct {
    DB         *bun.DB
    Registry   *Registry
    Config     *Config
    WithLocale func(ctx context.Context, lang string) context.Context
}
```

| Field | Description |
|-------|-------------|
| `DB` | Bun database connection (SQLite with WAL mode) |
| `Registry` | App registry for looking up other apps |
| `Config` | Parsed framework configuration |
| `WithLocale` | Function that returns a new context with the given locale set (provided by the i18n `Bundle`) |

## Optional

Apps can implement any combination of these interfaces. The framework detects them via type assertion and calls the appropriate methods during the boot sequence.

### Migratable

```go
type Migratable interface {
    MigrationFS() fs.FS
}
```

Provides an `fs.FS` containing `.up.sql` migration files at the root level. Called during startup before `Register()`. When using `//go:embed migrations`, you must strip the directory prefix:

```go
//go:embed migrations
var migrationFS embed.FS

func (a *App) MigrationFS() fs.FS {
    sub, _ := fs.Sub(migrationFS, "migrations")
    return sub
}
```

See the [Migrations guide](../guide/migrations.md) for details on file naming and tracking.

### HasRoutes

```go
type HasRoutes interface {
    Routes(r chi.Router)
}
```

Registers HTTP handlers on the Chi router. Called after all apps are registered.

```go
func (a *App) Routes(r chi.Router) {
    r.Route("/notes", func(r chi.Router) {
        r.Get("/", burrow.Handle(a.handleList))
        r.Get("/{id}", burrow.Handle(a.handleDetail))

        r.Group(func(r chi.Router) {
            r.Use(auth.RequireAuth())
            r.Post("/", burrow.Handle(a.handleCreate))
        })
    })
}
```

See the [Routing guide](../guide/routing.md) for details on handlers, URL parameters, and middleware.

### HasMiddleware

```go
type HasMiddleware interface {
    Middleware() []func(http.Handler) http.Handler
}
```

Returns middleware functions applied globally to the router. Applied in app registration order.

```go
func (a *App) Middleware() []func(http.Handler) http.Handler {
    return []func(http.Handler) http.Handler{
        a.sessionMiddleware,
    }
}
```

### HasNavItems

```go
type HasNavItems interface {
    NavItems() []NavItem
}
```

Returns navigation entries collected into the request context by the framework:

```go
type NavItem struct {
    Label     string
    LabelKey  string        // i18n message ID
    URL       string
    Icon      template.HTML // inline SVG, empty string for no icon
    Position  int
    AuthOnly  bool
    AdminOnly bool
}
```

```go
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
```

See the [Navigation guide](../guide/navigation.md) for positioning and ordering.

### HasTemplates

```go
type HasTemplates interface {
    TemplateFS() fs.FS
}
```

Returns an `fs.FS` containing `.html` template files. Templates must use `{{ define "appname/templatename" }}` blocks to namespace themselves. All template files from all apps are parsed into a single global `*template.Template` at boot time.

```go
//go:embed templates/*.html
var templateFS embed.FS

func (a *App) TemplateFS() fs.FS {
    sub, _ := fs.Sub(templateFS, "templates")
    return sub
}
```

See the [Layouts & Rendering guide](../guide/layouts.md) for details on template rendering and layout wrapping.

### HasFuncMap

```go
type HasFuncMap interface {
    FuncMap() template.FuncMap
}
```

Returns a static `template.FuncMap` added at parse time. Functions are available globally in all templates. The framework panics if two apps register the same function name.

!!! tip "Functions are global — don't register twice"
    Once an app registers a function, it is available in **all** templates across all apps. If your app depends on another app that already registers a function (e.g., icon functions), use it directly in your templates — do not re-register it in your own `FuncMap()`. Duplicate registration causes a panic.

    To avoid name collisions, prefix custom functions with your app name (e.g., `notesFormatDate` instead of `formatDate`). This is especially important for icon functions where a collision would silently swap one icon for another.

```go
func (a *App) FuncMap() template.FuncMap {
    return template.FuncMap{
        "formatDate": func(t time.Time) string {
            return t.Format("2006-01-02")
        },
    }
}
```

!!! warning "Reserved function names"
    The following names are already registered by the framework and contrib apps:
    `safeHTML`, `safeURL`, `safeAttr`, `itoa`, `lang`, `navItems`, `navLinks`, `staticURL`, `csrfToken`, `t`, `tData`, `tPlural`, `currentUser`, `isAuthenticated`, `add`, `sub`, `pageURL`, `pageLimit`, `pageNumbers`.
    Do not use these names in your own `FuncMap` — the server will panic at startup.

### HasRequestFuncMap

```go
type HasRequestFuncMap interface {
    RequestFuncMap(r *http.Request) template.FuncMap
}
```

Returns request-scoped template functions that are injected per-request via `template.Clone()`. Use this for functions that depend on the request context (e.g., current user, CSRF token, locale).

```go
func (a *App) RequestFuncMap(r *http.Request) template.FuncMap {
    return template.FuncMap{
        "currentUser": func() *User {
            return UserFromContext(r.Context())
        },
        "isAuthenticated": func() bool {
            return UserFromContext(r.Context()) != nil
        },
    }
}
```

### Configurable

```go
type Configurable interface {
    Flags(configSource func(key string) cli.ValueSource) []cli.Flag
    Configure(cmd *cli.Command) error
}
```

- `Flags()` returns CLI flags merged into the application's flag set. The `configSource` parameter enables TOML file sourcing — pass `nil` when no config file is used.
- `Configure()` is called after CLI parsing to read flag values

```go
func (a *App) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
    return []cli.Flag{
        &cli.IntFlag{
            Name:    "notes-page-size",
            Value:   20,
            Usage:   "Number of notes per page",
            Sources: burrow.FlagSources(configSource, "NOTES_PAGE_SIZE", "notes.page_size"),
        },
    }
}

func (a *App) Configure(cmd *cli.Command) error {
    a.pageSize = int(cmd.Int("notes-page-size"))
    return nil
}
```

See the [Configuration guide](../guide/configuration.md) for the three-tier config system.

### HasCLICommands

```go
type HasCLICommands interface {
    CLICommands() []*cli.Command
}
```

Returns CLI subcommands (e.g., `promote`, `demote`). Collect them with `srv.Registry().AllCLICommands()`.

```go
func (a *App) CLICommands() []*cli.Command {
    return []*cli.Command{
        {
            Name:  "seed-notes",
            Usage: "Create sample notes for testing",
            Action: func(ctx context.Context, cmd *cli.Command) error {
                return a.seedNotes(ctx)
            },
        },
    }
}
```

### Seedable

```go
type Seedable interface {
    Seed(ctx context.Context) error
}
```

Seeds the database with initial data. Called automatically during startup after migrations and app registration. Seeders run in app registration order and stop on the first error.

```go
func (a *App) Seed(ctx context.Context) error {
    count, _ := a.repo.CountCategories(ctx)
    if count > 0 {
        return nil // already seeded
    }
    return a.repo.CreateCategories(ctx, defaultCategories)
}
```

### HasStaticFiles

```go
type HasStaticFiles interface {
    StaticFS() (prefix string, fsys fs.FS)
}
```

Contributes static file assets that the `staticfiles` app collects and serves. The `prefix` namespaces files under the static URL path (e.g., prefix `"auth"` serves files at `/static/auth/...`). Files are content-hashed and cache-busted just like user-provided static files.

```go
//go:embed static
var staticFS embed.FS

func (a *App) StaticFS() (string, fs.FS) {
    sub, _ := fs.Sub(staticFS, "static")
    return "myapp", sub
}
```

### HasAdmin

```go
type HasAdmin interface {
    AdminRoutes(r chi.Router)
    AdminNavItems() []NavItem
}
```

Contributes admin panel routes and navigation items. `AdminRoutes` receives a Chi router already prefixed with `/admin` and protected by auth middleware. The `admin` contrib app discovers all `HasAdmin` implementations and mounts them.

```go
func (a *App) AdminRoutes(r chi.Router) {
    r.Get("/notes", burrow.Handle(a.adminListNotes))
    r.Get("/notes/{id}", burrow.Handle(a.adminNoteDetail))
}

func (a *App) AdminNavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {Label: "Notes", URL: "/admin/notes", Position: 30},
    }
}
```

See the [Admin contrib app](../contrib/admin.md) for the full admin panel setup.

### HasTranslations

```go
type HasTranslations interface {
    TranslationFS() fs.FS
}
```

Contributes translation files for the `i18n` app. The returned `fs.FS` must contain TOML files (e.g., `active.en.toml`, `active.de.toml`). The `i18n` app auto-discovers all `HasTranslations` implementations at startup.

```go
//go:embed translations
var translationFS embed.FS

func (a *App) TranslationFS() fs.FS { return translationFS }
```

See the [i18n guide](../guide/i18n.md) for translation file format and usage.

### HasDependencies

```go
type HasDependencies interface {
    Dependencies() []string
}
```

Returns app names that must be registered before this app. `NewServer` automatically sorts apps by dependencies. The registry panics at startup if any dependency is missing.

```go
func (a *App) Dependencies() []string { return []string{"session", "auth"} }
```

### HasJobs

```go
type HasJobs interface {
    RegisterJobs(q Queue)
}
```

Registers background job handlers with the job queue. The queue implementation (e.g., `contrib/jobs`) discovers all `HasJobs` apps during `Configure()` and calls `RegisterJobs` on each one. Use `q.Handle()` to register named handlers:

```go
func (a *App) RegisterJobs(q burrow.Queue) {
    q.Handle("notes.cleanup", burrow.JobHandlerFunc(a.handleCleanup))
}
```

### HasShutdown

```go
type HasShutdown interface {
    Shutdown(ctx context.Context) error
}
```

Performs cleanup during graceful shutdown (e.g., stopping background goroutines, flushing buffers). Called in **reverse** registration order before the HTTP server stops. Errors are logged but do not prevent other apps from shutting down. The context carries the server's shutdown timeout.

```go
func (a *App) Shutdown(_ context.Context) error {
    close(a.stopCh) // signal background worker to stop
    return nil
}
```
