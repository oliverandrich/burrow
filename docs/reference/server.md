# Server & Registry

## Server

The `Server` is the main entry point for the framework. It holds the app registry and orchestrates the boot sequence.

### Creating a Server

```go
srv := burrow.NewServer(
    session.New(),
    auth.New(),
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
func (s *Server) SetLayout(fn LayoutFunc)
```

Configures the app layout function. Call before `Run()`.

#### Registry

```go
func (s *Server) Registry() *Registry
```

Returns the server's app registry for direct access.

#### Flags

```go
func (s *Server) Flags(configSource func(key string) cli.ValueSource) []cli.Flag
```

Returns all CLI flags: core framework flags merged with flags from all `Configurable` apps. Pass a config source function to enable TOML file sourcing, or `nil` for CLI+ENV only.

#### Run

```go
func (s *Server) Run(ctx context.Context, cmd *cli.Command) error
```

Boots and starts the server. This is a `cli.ActionFunc` — pass it directly to `cli.Command.Action`.

### Boot Sequence

When `Run()` is called, the following happens in order:

1. **Parse config** — reads CLI flags, env vars, and TOML into a `Config` struct
2. **Open database** — connects to SQLite with WAL mode, foreign keys, and connection pool
3. **Run migrations** — calls `RunAppMigrations` for every `Migratable` app
4. **Register apps** — calls `Register()` on each app with the shared `AppConfig`
5. **Configure apps** — calls `Configure()` on each `Configurable` app
6. **Create router** — sets up Chi with core middleware (request logger, request ID, gzip, body limit)
7. **Inject nav items and layout** — collects nav items from all `HasNavItems` apps and the layout function into request context
8. **Register middleware** — applies middleware from all `HasMiddleware` apps
9. **Register routes** — calls `Routes()` on all `HasRoutes` apps
10. **Start HTTP server** — listens on the configured address with graceful shutdown and zero-downtime restart via SIGHUP (see [Deployment Guide](../guide/deployment.md))

!!! note "Logging"
    The framework uses `slog.Default()` for all logging. Configure your preferred logger (text, JSON, [tint](https://github.com/lmittmann/tint), etc.) by calling `slog.SetDefault()` before starting the server.

## Registry

The `Registry` manages registered apps and provides access to their capabilities.

### Methods

#### Add

```go
func (r *Registry) Add(app App)
```

Registers an app. Panics on duplicate names or missing dependencies.

#### Get

```go
func (r *Registry) Get(name string) (App, bool)
```

Returns the app with the given name, or `false` if not found. Use with type assertions to access app-specific methods.

#### Apps

```go
func (r *Registry) Apps() []App
```

Returns all registered apps in registration order.

#### RunMigrations

```go
func (r *Registry) RunMigrations(ctx context.Context, db *bun.DB) error
```

Runs migrations for all `Migratable` apps.

#### AllNavItems

```go
func (r *Registry) AllNavItems() []NavItem
```

Collects and sorts nav items from all `HasNavItems` apps by position.

#### RegisterMiddleware

```go
func (r *Registry) RegisterMiddleware(router chi.Router)
```

Applies middleware from all `HasMiddleware` apps to the router.

#### RegisterRoutes

```go
func (r *Registry) RegisterRoutes(router chi.Router)
```

Calls `Routes()` on all `HasRoutes` apps.

#### AllFlags

```go
func (r *Registry) AllFlags(configSource func(key string) cli.ValueSource) []cli.Flag
```

Collects CLI flags from all `Configurable` apps. Pass `nil` for CLI+ENV only.

#### Configure

```go
func (r *Registry) Configure(cmd *cli.Command) error
```

Calls `Configure()` on each `Configurable` app.

#### AllCLICommands

```go
func (r *Registry) AllCLICommands() []*cli.Command
```

Collects CLI subcommands from all `HasCLICommands` apps.

#### Seed

```go
func (r *Registry) Seed(ctx context.Context) error
```

Calls `Seed()` on each `Seedable` app in order.

## RenderTemplate

```go
func RenderTemplate(w http.ResponseWriter, r *http.Request, statusCode int, name string, data map[string]any) error
```

Renders a named template into the HTTP response. If the request has an `HX-Request` header (htmx), the fragment is returned directly. Otherwise, it is wrapped in the layout function from context (if set).
