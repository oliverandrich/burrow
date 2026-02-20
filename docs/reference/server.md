# Server & Registry

## Server

The `Server` is the main entry point for the framework. It holds the app registry and orchestrates the boot sequence.

### Creating a Server

```go
srv := core.NewServer(
    &session.App{},
    auth.New(nil),
    &healthcheck.App{},
    myApp,
)
```

Apps are registered in the order provided. Dependencies must appear before the apps that depend on them.

### Methods

#### NewServer

```go
func NewServer(apps ...App) *Server
```

Creates a server and registers all given apps in order.

#### SetLayouts

```go
func (s *Server) SetLayouts(l Layouts)
```

Configures the app and admin layout functions. Call before `Run()`.

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
2. **Set up logging** — configures `slog` with the specified level and format
3. **Open database** — connects to SQLite with WAL mode, foreign keys, and connection pool
4. **Run migrations** — calls `RunAppMigrations` for every `Migratable` app
5. **Register apps** — calls `Register()` on each app with the shared `AppConfig`
6. **Configure apps** — calls `Configure()` on each `Configurable` app
7. **Create Echo** — sets up the HTTP router with core middleware (recover, request ID, gzip, body limit)
8. **Inject nav items** — collects nav items from all `HasNavItems` apps into request context
9. **Register middleware** — applies middleware from all `HasMiddleware` apps
10. **Register routes** — calls `Routes()` on all `HasRoutes` apps
11. **Start HTTP server** — listens on the configured address with graceful shutdown

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
func (r *Registry) RegisterMiddleware(e *echo.Echo)
```

Applies middleware from all `HasMiddleware` apps to the Echo instance.

#### RegisterRoutes

```go
func (r *Registry) RegisterRoutes(e *echo.Echo)
```

Calls `Routes()` on all `HasRoutes` apps.

#### AllFlags

```go
func (r *Registry) AllFlags() []cli.Flag
```

Collects CLI flags from all `Configurable` apps.

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

## Render

```go
func Render(c *echo.Context, statusCode int, component templ.Component) error
```

Renders a Templ component into the HTTP response with the given status code. Uses a pooled buffer for efficiency.
