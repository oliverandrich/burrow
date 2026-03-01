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
}
```

| Field | Description |
|-------|-------------|
| `DB` | Bun database connection (SQLite with WAL mode) |
| `Registry` | App registry for looking up other apps |
| `Config` | Parsed framework configuration |

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
    LabelKey  string          // i18n message ID
    URL       string
    Icon      templ.Component // nil = no icon
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

Seeds the database with initial data. Called automatically during startup after migrations and app registration. Seeders run in app registration order and stop on the first error.

### HasStaticFiles

```go
type HasStaticFiles interface {
    StaticFS() (prefix string, fsys fs.FS)
}
```

Contributes static file assets that the `staticfiles` app collects and serves. The `prefix` namespaces files under the static URL path (e.g., prefix `"admin"` serves files at `/static/admin/...`). Files are content-hashed and cache-busted just like user-provided static files.

### HasAdmin

```go
type HasAdmin interface {
    AdminRoutes(r chi.Router)
    AdminNavItems() []NavItem
}
```

Contributes admin panel routes and navigation items. `AdminRoutes` receives a Chi router already prefixed with `/admin` and protected by auth middleware. The `admin` contrib app discovers all `HasAdmin` implementations and mounts them.

### HasTranslations

```go
type HasTranslations interface {
    TranslationFS() fs.FS
}
```

Contributes translation files for the `i18n` app. The returned `fs.FS` must contain a `translations/` directory with TOML files (e.g., `translations/active.en.toml`). The `i18n` app auto-discovers all `HasTranslations` implementations at startup.

### HasDependencies

```go
type HasDependencies interface {
    Dependencies() []string
}
```

Returns app names that must be registered before this app. The registry panics at startup if any are missing.
