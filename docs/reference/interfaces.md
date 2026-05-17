# Core Interfaces

All interfaces are defined in the `burrow` package (`github.com/oliverandrich/burrow`).

## Required

### App

Every app must implement this interface:

```go
type App interface {
    Name() string
}
```

- `Name()` returns a unique identifier for the app (e.g., `"auth"`, `"notes"`)

Apps that don't need configuration only need `Name()`. Setup logic (initialising repositories, services, etc.) goes in `Configure()` — see [Configurable](#configurable).

```go
type myApp struct{}

func (a *myApp) Name() string { return "notes" }
```

### AppConfig

Passed to every app's `Configure` method:

```go
type AppConfig struct {
    DB         *den.DB
    Registry   *Registry
    Config     *Config
    WithLocale func(ctx context.Context, lang string) context.Context
}
```

| Field | Description |
|-------|-------------|
| `DB` | Den database connection (SQLite with WAL mode, or PostgreSQL) |
| `Registry` | App registry for looking up other apps |
| `Config` | Parsed framework configuration |
| `WithLocale` | Function that returns a new context with the given locale set (provided by the i18n `Bundle`) |

## Optional

Apps can implement any combination of these interfaces. The framework detects them via type assertion and calls the appropriate methods during the boot sequence.

### HasDocuments

```go
type HasDocuments interface {
    Documents() []document.Document
}
```

Returns a slice of document type instances that Den should register. Called during startup before `Configure()`. Den inspects each type's struct tags and creates or updates the underlying collections and indexes automatically:

```go
func (a *App) Documents() []document.Document {
    return []document.Document{&Note{}, &Tag{}}
}
```

See the [Migrations guide](../guide/migrations.md) for details on schema management.

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
    LabelKey  string // i18n message ID
    URL       string
    Icon      string // template-define name (e.g. "auth/icon_people"), empty for no icon
    Position  int
    AuthOnly  bool
    AdminOnly bool
}
```

Layouts render `Icon` via the `{{ icon "..." }}` template function, which looks up the named template define (see [Template Functions](template-functions.md#core-functions)). Each contrib owns its icons in `templates/icons.html` as `{{ define "<app>/icon_<name>" }}` blocks.

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
    The following names are owned by the framework or shipped by standard contribs. Even if your build doesn't register the providing contrib, avoid these names in your own `FuncMap` — the server will panic at startup once the contrib is added.

    Always available (core base / always-on i18n bundle):
    `safeHTML`, `safeURL`, `safeAttr`, `add`, `sub`, `pageURL`, `pageNumbers`, `icon`, `dict`, `lang`, `t`, `tData`, `tPlural`, `navItems`, `navLinks`. Also `mediaURL` when Den is opened with a Storage.

    Reserved for standard contribs (registered only when the contrib is registered; templates using them otherwise fail to parse at boot):
    `staticURL` (staticfiles), `csrfToken` / `csrfField` / `csrfHxHeaders` (csrf), `messages` (messages), `currentUser` / `isAuthenticated` / `authLogo` / `credName` / `deref` (auth), `naturaltime` / `intcomma` / `filesizeformat` (humanize).

### HasRequestFuncMap

```go
type HasRequestFuncMap interface {
    RequestFuncMap(ctx context.Context) template.FuncMap
}
```

Returns context-scoped template functions that are injected per-request via `template.Clone()`. Use this for functions that depend on the context (e.g., current user, CSRF token, locale). The `context.Context` parameter enables template rendering both inside HTTP handlers (where the context comes from the request) and outside them (background jobs, SSE broadcasts) via `RenderFragment`.

```go
func (a *App) RequestFuncMap(ctx context.Context) template.FuncMap {
    return template.FuncMap{
        "currentUser": func() *User {
            return CurrentUser(ctx)
        },
        "isAuthenticated": func() bool {
            return CurrentUser(ctx) != nil
        },
    }
}
```

### HasFlags

```go
type HasFlags interface {
    Flags(configSource func(key string) cli.ValueSource) []cli.Flag
}
```

Returns CLI flags merged into the application's flag set. The `configSource` parameter enables TOML file sourcing — pass `nil` when no config file is used.

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
```

### Configurable

```go
type Configurable interface {
    Configure(cfg *AppConfig, cmd *cli.Command) error
}
```

Called after CLI parsing to read flag values and initialise the app. Receives the shared `AppConfig` (database, registry, config) and the parsed CLI command for reading flag values. All setup logic that needs database access or flag values belongs here.

```go
func (a *App) Configure(cfg *burrow.AppConfig, cmd *cli.Command) error {
    a.repo = NewRepository(cfg.DB)
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

Returns CLI subcommands (e.g., `promote`, `demote`). Wire them into the top-level `cli.Command` via `srv.CLICommands()`:

```go
cmd := &cli.Command{
    Name:     "myapp",
    Flags:    srv.Flags(nil),
    Action:   srv.Run,
    Commands: srv.CLICommands(), // boots DB + Configure before each subcommand
}
```

`srv.CLICommands()` wraps each subcommand's `Action` with the server's boot sequence (open DB, run `Configure()` on all apps) and tears it down afterwards, so a subcommand like `promote alice` runs against a fully-configured app graph. Use `srv.Registry().AllCLICommands()` only as a low-level escape hatch when you need the raw, unwrapped commands and want to manage the boot lifecycle yourself.

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

Seeds the database with initial data. Only runs when the server is started with the `--seed` flag (or `SEED=true`). Seeders run in app registration order and stop on the first error. Implementations should be idempotent — safe to call multiple times without creating duplicates.

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

### AdminAuth

```go
type AdminAuth interface {
    RequireAuth() func(http.Handler) http.Handler
    RequireAdmin() func(http.Handler) http.Handler
}
```

Provides authentication and authorization middleware for the admin panel. The `admin` contrib app discovers an `AdminAuth` provider from the registry during `Configure` and uses its middleware to protect `/admin` routes. `contrib/auth` implements this interface — custom auth systems can provide their own implementation.

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

Registers background job handlers with the job queue. The queue implementation (e.g., `contrib/jobs`) discovers all `HasJobs` apps during its `PostConfigure()` phase and calls `RegisterJobs` on each one. Because `PostConfigure()` runs after all `Configure()` calls, your app can safely use state set in `Configure()` inside `RegisterJobs`.

Use typed task definitions for compile-time safety, or the raw `q.Handle()` for dynamic job types:

```go
// Typed (recommended):
var cleanupTask = burrow.DefineTask("notes.cleanup", handleCleanup)

func (a *App) RegisterJobs(q burrow.Queue) {
    cleanupTask.Register(q)
}

// Raw:
func (a *App) RegisterJobs(q burrow.Queue) {
    q.Handle("notes.cleanup", a.handleCleanup)
    a.jobs = q // store as burrow.Enqueuer for later enqueueing
}
```

### PostConfigurable

```go
type PostConfigurable interface {
    PostConfigure(cfg *AppConfig, cmd *cli.Command) error
}
```

Runs a second configuration pass after all `Configurable.Configure()` calls have completed. This is useful when an app needs to interact with other apps' state that is only available after `Configure()` — for example, `contrib/jobs` uses `PostConfigure()` to discover and register `HasJobs` handlers from other apps.

Most apps do not need this interface. Prefer `Configurable` unless you specifically need cross-app coordination that depends on post-Configure state.

### Startable

```go
type Startable interface {
    Start(srv *Server) error
}
```

Called after the full boot sequence completes — templates built, middleware and routes registered — but before the HTTP listener starts. This is the counterpart to `HasShutdown`: use `Startable` to launch background processes and `HasShutdown` to stop them.

The `*Server` parameter gives access to server resources like `TemplateExecutor()` that are only available after boot. For example, `contrib/jobs` implements `Startable` to create its worker pool with the template executor, so job handlers can use `RenderFragment`:

```go
func (a *App) Start(srv *burrow.Server) error {
    a.worker = NewWorker(a.repo, a.handlers, a.workerCfg, srv.TemplateExecutor())
    ctx, cancel := context.WithCancel(context.Background())
    a.cancelFunc = cancel
    go a.worker.Start(ctx)
    return nil
}
```

Most apps do not need this interface. Use it only when you need to start background goroutines that depend on the fully initialized server.

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

### ReadinessChecker

```go
type ReadinessChecker interface {
    ReadinessCheck(ctx context.Context) error
}
```

Contributes to the readiness probe at `/healthz/ready` (provided by the [healthcheck](../contrib/healthcheck.md) app). Return `nil` when the app is ready to serve traffic, or an error describing what is not ready. The healthcheck app iterates all registered `ReadinessChecker` apps and reports their status.

```go
func (a *App) ReadinessCheck(ctx context.Context) error {
    if err := a.pool.Ping(ctx); err != nil {
        return fmt.Errorf("connection pool: %w", err)
    }
    return nil
}
```
