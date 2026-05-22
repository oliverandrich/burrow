# Server & Registry

## Server

The `Server` is the main entry point for the framework. It holds the app registry and orchestrates the boot sequence.

### Creating a Server

```go
srv := burrow.NewServer(
    session.New(),
    auth.New[auth.EmptyProfile](),
    healthcheck.New(),
    myApp,
)
```

Apps are automatically sorted by their `HasDependencies` declarations — you can list them in any order.

### Methods

#### NewServer

```go
func NewServer(apps ...App) *Server
```

Creates a server and registers all given apps in order.

#### SetLayout

```go
func (s *Server) SetLayout(name string)
```

Configures the app layout template name. The name must refer to a template in the global template set (contributed by a `HasTemplates` app). Call before `Run()`.

#### Registry

```go
func (s *Server) Registry() *Registry
```

Returns the server's app registry for direct access.

#### Flags

```go
func (s *Server) Flags(configSource func(key string) cli.ValueSource) []cli.Flag
```

Returns all CLI flags: core framework flags merged with flags from all `HasFlags` apps. Pass a config source function to enable TOML file sourcing, or `nil` for CLI+ENV only.

#### Run

```go
func (s *Server) Run(ctx context.Context, cmd *cli.Command) error
```

Boots and starts the HTTP server. This is a `cli.ActionFunc` — pass it directly to `cli.Command.Action`.

#### CLICommands

```go
func (s *Server) CLICommands() []*cli.Command
```

Returns the CLI subcommands from all `HasCLICommands` apps, each wrapped to run inside the framework's boot lifecycle. The wrapped `Action` opens the database, runs `Configure()` on every app, then invokes the original Action; the database is closed when it returns. Use this in place of `srv.Registry().AllCLICommands()` when wiring contrib subcommands like `auth set-role`:

```go
cmd := &cli.Command{
    Flags:    srv.Flags(nil),
    Action:   srv.Run,
    Commands: srv.CLICommands(),
}
```

Without the wrapping, contrib subcommands would fire against uninitialised apps and fail with errors like `auth app not initialized`. `AllCLICommands` remains available on the registry as a low-level escape hatch when you want to manage the boot lifecycle yourself.

#### TemplateExecutor

```go
type TemplateExecutor func(ctx context.Context, name string, data map[string]any) (template.HTML, error)

func (s *Server) TemplateExecutor() TemplateExecutor
```

Returns the server's template executor. Use it after boot to render templates outside an HTTP handler — for example, from a background job or an SSE broadcast. Pair with [`burrow.WithTemplateExecutor`](context-helpers.md#withtemplateexecutor) to inject it into a context that `burrow.Render` / `burrow.RenderFragment` can pick up. Returns `nil` before templates have been built (i.e. before `Run`).

```go
func (a *App) Start(srv *burrow.Server) error {
    a.worker = NewWorker(a.repo, srv.TemplateExecutor())
    // worker calls burrow.RenderFragment with a context carrying the executor
    return nil
}
```

### Boot Sequence

`Server.Run` shares its boot phase with `Server.CLICommands` (both call the same internal `boot` helper) so that contrib subcommands run with the same fully-configured app graph as the HTTP server. The full ordering when `Run()` fires:

**Boot phase** (shared with `CLICommands`):

1. **Parse config** — builds `*Config` from CLI flags, env vars, and TOML (`NewConfig(cmd)`)
2. **Validate TLS** — checks `--tls-*` flags are coherent
3. **Resolve base URL** — falls back to host/port if `--base-url` is unset
4. **Create i18n bundle** — `i18n.NewBundle(defaultLang, supportedLangs)`; bundle is always present so `{{ t "..." }}` works even without `HasTranslations` apps
5. **Open storage** — opens `den.Storage` for the `--storage-dsn` (skipped when empty)
6. **Open database** — `OpenDB(ctx, dsn, den.WithStorage(...))` connects to Den (SQLite with WAL, or PostgreSQL)
7. **Register documents** — `Registry.RegisterDocuments` calls `Documents()` on every `HasDocuments` app and hands them to `den.Register`
8. **Build `AppConfig`** — `DB`, `Registry`, `Config`, `WithLocale` ready for `Configure`
9. **Load translations** — for each `HasTranslations` app, `bundle.AddTranslations(app.TranslationFS())`
10. **Configure + PostConfigure** — `Registry.Configure(cfg, cmd)` runs `Configure()` on every `Configurable` app, then a second pass runs `PostConfigure()` on every `PostConfigurable` (e.g. `contrib/jobs` discovers `HasJobs` handlers here, after all `Configure()` calls have run)
11. **Run migrations** — `Registry.RunMigrations` collects every `HasMigrations` app's migrations, namespaces each version as `{app.Name()}/{version}`, and calls `migrate.Up` once. Each pending migration runs in its own transaction; applied versions are tracked in the `_den_migrations` collection so re-boots are no-ops. A failing migration aborts boot. See [Database Migrations](../guide/migrations.md).

**Run-only phase** (HTTP server):

12. **Register request-scoped template providers** — core registers `i18n.Bundle.RequestFuncMap` and `coreRequestFuncMap` (`navItems`, `navLinks`) before templates are parsed
13. **Build templates** — collects `.html` files from all `HasTemplates` apps and `FuncMap()` from all `HasFuncMap` apps; parses into a single `*template.Template`. Per-request `HasRequestFuncMap` stubs are registered here too so templates parse cleanly
14. **Create router** — Chi router with core middleware: request logger, request ID, gzip, body-size limit, locale middleware
15. **Inject context middleware** — nav items (from `HasNavItems`), layout name, template executor
16. **Apply contrib middleware** — `Registry.RegisterMiddleware` runs every `HasMiddleware` app
17. **Apply contrib routes** — `Registry.RegisterRoutes` runs every `HasRoutes` app; default 404 / 405 handlers register last
18. **Start background processes** — for every `Startable` app, `Start(srv)` runs (e.g. `contrib/jobs` launches its worker pool with the template executor)
19. **Start HTTP server** — listens on the configured address with graceful shutdown and zero-downtime restart via SIGHUP (see [Deployment Guide](../guide/deployment.md))

!!! note "Logging"
    The framework uses `slog.Default()` for all logging. Configure your preferred logger (text, JSON, [tint](https://github.com/lmittmann/tint), etc.) by calling `slog.SetDefault()` before starting the server.

### Why urfave/cli?

`Server.Run()` is a `cli.ActionFunc` by design. The framework uses `urfave/cli` throughout — `NewConfig()` reads values from `*cli.Command`, `Configure()` passes the `AppConfig` and command to each app, and flags define the three-layer config cascade (CLI flags → ENV vars → TOML file).

This means you cannot start the server with a different CLI framework (cobra, kong, etc.) or without one. This is intentional: the tight integration gives every app a consistent way to declare and read configuration without boilerplate. The trade-off is that `urfave/cli` is a load-bearing dependency — it's part of the framework contract, not a swappable implementation detail.

## Registry

The `registry` package holds the apps that make up a server — pure storage, no lifecycle. The `Server` constructs one in `NewServer` and exposes it as `cfg.Registry` to every app's `Configure` method. Lifecycle orchestration (Configure, RegisterMiddleware, RegisterRoutes, RunMigrations, Shutdown) lives inside `burrow/server` as private helpers and runs automatically during boot — application code does not call those helpers directly.

`burrow.Registry` is a type alias for `*registry.Registry`; both names refer to the same type. `burrow.App` is a type alias for `app.App`, which is itself an alias for `registry.App`.

### Storage API (`package registry`)

#### New

```go
func registry.New() *Registry
```

Creates an empty registry. `NewServer` calls this internally; application code typically does not.

#### Add

```go
func registry.Add(reg *Registry, app App)
```

Registers an app. Panics on a duplicate name or a missing dependency declared via `HasDependencies`.

#### Get

```go
func registry.Get[T App](reg *Registry) (T, bool)
```

Returns the unique app of type `T`, or the zero value and `false` when no app of type `T` is registered or when more than one is. The idiomatic shape for **Optional-Service** lookups where graceful degradation is acceptable.

#### MustGet

```go
func registry.MustGet[T App](reg *Registry) T
```

Returns the unique app of type `T`. Panics with a message naming the type when no app of type `T` is registered, when more than one is registered, or when the registered app has a different concrete type. The idiomatic shape for **Hard-Dependency** lookups when the provider is declared in `Dependencies()`.

#### GetByName

```go
func registry.GetByName(reg *Registry, name string) (App, bool)
```

Returns the app with the given name as `App`, or `false` if not found. Use when you have the name but not a typed handle.

#### MustGetByName

```go
func registry.MustGetByName(reg *Registry, name string) App
```

Like `GetByName`, but panics when the named app is not registered.

#### Apps

```go
func registry.Apps(reg *Registry) []App
```

Returns a copy of all registered apps in registration order. Combine with a type assertion against an exported interface for **Provider-Discovery** patterns — e.g. `contrib/admin` iterates `registry.Apps(cfg.Registry)` to find any app implementing `burrow.AdminAuth`.

See [Inter-App Communication](../guide/inter-app-communication.md) for end-to-end usage patterns.

## Render

```go
func Render(w http.ResponseWriter, r *http.Request, statusCode int, name string, data map[string]any) error
```

Renders a named template into the HTTP response. If the request has an `HX-Request` header (htmx), the fragment is returned directly. Otherwise, it is wrapped in the layout template from context (if set).
