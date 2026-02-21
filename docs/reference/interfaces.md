# Core Interfaces

All interfaces are defined in the `burrow` package (`codeberg.org/oliverandrich/burrow`).

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
- `Register()` receives the shared `AppConfig` for initialization

### AppConfig

Passed to every app's `Register` method:

```go
type AppConfig struct {
    DB       *bun.DB
    Registry *Registry
    Config   *Config
    Layouts  Layouts
}
```

| Field | Description |
|-------|-------------|
| `DB` | Bun database connection (SQLite with WAL mode) |
| `Registry` | App registry for looking up other apps |
| `Config` | Parsed framework configuration |
| `Layouts` | Layout functions for rendering |

## Optional

Apps can implement any combination of these interfaces. The framework detects them via type assertion and calls the appropriate methods during the boot sequence.

### Migratable

```go
type Migratable interface {
    MigrationFS() fs.FS
}
```

Provides an `fs.FS` containing `.up.sql` migration files. Called during startup before `Register()`.

### HasRoutes

```go
type HasRoutes interface {
    Routes(r chi.Router)
}
```

Registers HTTP handlers on the Chi router. Called after all apps are registered.

### HasMiddleware

```go
type HasMiddleware interface {
    Middleware() []func(http.Handler) http.Handler
}
```

Returns middleware functions applied globally to the router. Applied in app registration order.

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
    URL       string
    Icon      string
    Position  int
    AuthOnly  bool
    AdminOnly bool
}
```

### Configurable

```go
type Configurable interface {
    Flags() []cli.Flag
    Configure(cmd *cli.Command) error
}
```

- `Flags()` returns CLI flags merged into the application's flag set
- `Configure()` is called after CLI parsing to read flag values

### HasCLICommands

```go
type HasCLICommands interface {
    CLICommands() []*cli.Command
}
```

Returns CLI subcommands (e.g., `promote`, `demote`). Collect them with `srv.Registry().AllCLICommands()`.

### Seedable

```go
type Seedable interface {
    Seed(ctx context.Context) error
}
```

Seeds the database with initial data. Call `registry.Seed(ctx)` to run all seeders.

### HasDependencies

```go
type HasDependencies interface {
    Dependencies() []string
}
```

Returns app names that must be registered before this app. The registry panics at startup if any are missing.
