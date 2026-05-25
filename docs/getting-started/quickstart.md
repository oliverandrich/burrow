# Quick Start

Build a minimal working application with a custom homepage and health check endpoint.

!!! tip "Faster start"
    For a ready-to-run project with a contrib stack, live reload, CI,
    and goreleaser config already wired up, install the `burrow` CLI
    and run `burrow new`:

    ```bash
    go install github.com/oliverandrich/burrow/cmd/burrow@latest
    burrow new myapp --module github.com/you/myapp
    cd myapp && mise run setup && mise run dev
    ```

    Without `mise`, the scaffold prints a `go mod tidy && go run ./cmd/myapp` fallback.

    See [Scaffold](scaffold.md) for a file-by-file walkthrough of what
    `burrow new` generates and the `mise` tasks it ships, or
    [Tooling](tooling.md) for the CLI flag reference.

## 1. Create the Project

```bash
mkdir myapp && cd myapp
go mod init myapp
go get github.com/oliverandrich/burrow@latest
```

## 2. Write main.go

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"

    "github.com/oliverandrich/burrow"
    _ "github.com/oliverandrich/den/backend/sqlite" // register sqlite:// scheme
    "github.com/go-chi/chi/v5"
    "github.com/urfave/cli/v3"
)

// homeApp is a minimal app with a single route.
type homeApp struct{}

func (a *homeApp) Name() string { return "home" }
func (a *homeApp) Routes(r chi.Router) {
    r.Get("/", burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
        return burrow.Text(w, http.StatusOK, "Hello from Burrow!")
    }))
}

func main() {
    srv := burrow.NewServer(
        &homeApp{},
    )

    cmd := &cli.Command{
        Name:    "myapp",
        Usage:   "My application",
        Version: "0.1.0",
        Flags:   srv.Flags(nil),
        Action:  srv.Run,
    }

    if err := cmd.Run(context.Background(), os.Args); err != nil {
        log.Fatal(err)
    }
}
```

This shows the core pattern:

- Every app implements `Name()` (the `App` interface)
- Apps that serve HTTP routes also implement `Routes(r chi.Router)` (the `HasRoutes` interface)
- Handlers return `error` instead of silently failing — `burrow.Handle()` wraps them and handles errors automatically: `*HTTPError` renders an error page with the appropriate status code, any other error becomes a logged 500
- `srv.Flags(nil)` adds built-in CLI flags (`--host`, `--port`, `--database-dsn`, etc.)
- The blank-import `_ "…/den/backend/sqlite"` registers the `sqlite://` scheme. Use `…/den/backend/postgres` instead (or alongside) when running against PostgreSQL — Burrow does not pull either backend in by default, so binaries only link the engine they actually use.

No layout needed yet — that comes later when you want to render HTML templates (see the [Tutorial](../tutorial/index.md)).

## 3. Run It

```bash
go mod tidy
go run main.go
```

The server starts on `localhost:8080` with a SQLite database at `app.db` (auto-created with WAL mode).

## 4. Test It

```bash
curl http://localhost:8080/
# Hello from Burrow!
```

## 5. Configure It

Override defaults with CLI flags, environment variables, or a TOML config file:

=== "CLI Flags"

    ```bash
    go run main.go --port 3000 --database-dsn sqlite:///myapp.db
    ```

=== "Environment Variables"

    ```bash
    PORT=3000 DATABASE_DSN=sqlite:///myapp.db go run main.go
    ```

See [Configuration](../guide/configuration.md) for the full reference.

## Next Steps

- [Project Structure](project-structure.md) — recommended directory layout
- [Creating an App](../guide/creating-an-app.md) — build a custom app
- [Tutorial](../tutorial/index.md) — build a complete polls app step by step
- [Auth](../contrib/auth.md) — set up WebAuthn passkey authentication
