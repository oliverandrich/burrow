# Burrow

A Go web framework library built on [Chi](https://go-chi.io/), [Bun](https://bun.uptrace.dev/)/SQLite, and [Templ](https://templ.guide/). Designed around composable apps with a Django-inspired architecture.

## Features

- **App-based architecture** — build your application from composable, self-contained apps
- **Pure Go SQLite** — no CGO required (`CGO_ENABLED=0`), cross-compiles anywhere
- **Per-app migrations** — each app manages its own SQL migrations
- **CSS-agnostic** — bring your own CSS framework (Bootstrap, Tailwind, etc.)
- **Layout system** — app layout via server, admin layout via admin package
- **CLI configuration** — flags, environment variables, and TOML config via [urfave/cli](https://github.com/urfave/cli)
- **Contrib apps** — auth (WebAuthn/passkeys), sessions, i18n, admin, healthcheck, static files

## Quick Start

```go
package main

import (
    "context"
    "log"
    "os"

    "codeberg.org/oliverandrich/burrow"
    "codeberg.org/oliverandrich/burrow/contrib/auth"
    "codeberg.org/oliverandrich/burrow/contrib/healthcheck"
    "codeberg.org/oliverandrich/burrow/contrib/session"
    "github.com/urfave/cli/v3"
)

func main() {
    srv := burrow.NewServer(
        &session.App{},
        auth.New(nil),
        &healthcheck.App{},
    )

    cmd := &cli.Command{
        Name:   "myapp",
        Flags:  srv.Flags(nil),
        Action: srv.Run,
    }

    if err := cmd.Run(context.Background(), os.Args); err != nil {
        log.Fatal(err)
    }
}
```

## Architecture

```
contrib/        Reusable apps
  auth/         WebAuthn passkeys, recovery codes, email verification
  session/      Cookie-based sessions
  i18n/         Locale detection and translations
  admin/        Admin panel
  healthcheck/  /healthz endpoint
  staticfiles/  Static file serving with content-hashed URLs
example/        Example application with a notes app
```

### The App Interface

Every app implements `burrow.App`:

```go
type App interface {
    Name() string
    Register(cfg *AppConfig) error
}
```

Apps can optionally implement additional interfaces:

| Interface | Purpose |
|---|---|
| `Migratable` | Provide embedded SQL migrations |
| `HasRoutes` | Register HTTP routes |
| `HasMiddleware` | Contribute middleware |
| `HasNavItems` | Contribute navigation items |
| `Configurable` | Define CLI flags and read configuration |
| `HasCLICommands` | Contribute CLI subcommands |
| `Seedable` | Seed the database with initial data |

### Layouts

The app layout wraps user-facing pages:

```go
srv.SetLayout(appLayout)
```

The admin layout is owned by the admin package:

```go
admin.New(adminLayout)
```

A `LayoutFunc` receives a page title and content, and returns a wrapped component:

```go
type LayoutFunc func(title string, content templ.Component) templ.Component
```

Layouts access framework values from the request context:

```go
burrow.NavItems(ctx)    // Navigation items from all apps
burrow.Layout(ctx)      // App layout function
csrf.Token(ctx)         // CSRF token for forms
```

### Configuration

Configuration is resolved in order: CLI flags > environment variables > TOML file.

Core flags include `--host`, `--port`, `--database-dsn`, `--log-level`, `--log-format`, `--tls-mode`, and more. Apps can contribute their own flags via the `Configurable` interface.

### Migrations

Apps embed their SQL migrations and implement `Migratable`:

```go
//go:embed migrations
var migrationFS embed.FS

func (a *App) MigrationFS() fs.FS {
    sub, _ := fs.Sub(migrationFS, "migrations")
    return sub
}
```

Migrations are tracked per-app in the `_migrations` table and run automatically on startup.

## Development

```bash
just setup          # Check that all required dev tools are installed
just test           # Run all tests
just lint           # Run golangci-lint
just fmt            # Format code
just coverage       # Generate coverage report
just tidy           # Tidy module dependencies
just example        # Run the example application
```

Requires Go 1.25+. Run `just setup` to verify your dev environment.

## License

See [LICENSE](LICENSE).
