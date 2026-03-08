# Quick Start

Build a working application with session management, a health check endpoint, and a custom homepage.

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
    "github.com/go-chi/chi/v5"
    "github.com/urfave/cli/v3"
)

// homeApp is a minimal app with a single route.
type homeApp struct{}

func (a *homeApp) Name() string                        { return "home" }
func (a *homeApp) Register(_ *burrow.AppConfig) error   { return nil }
func (a *homeApp) Routes(r chi.Router) {
    r.Method("GET", "/", burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
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

This shows the core pattern: define an app, implement `Routes()`, and use `burrow.Handle()` to write handlers that return errors. No layout needed yet — that comes later when you want to render HTML templates (see the [Tutorial](../tutorial/index.md)).

## 3. Run It

```bash
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
    go run main.go --port 3000 --database-dsn ./myapp.db
    ```

=== "Environment Variables"

    ```bash
    PORT=3000 DATABASE_DSN=./myapp.db go run main.go
    ```

See [Configuration](../guide/configuration.md) for the full reference.

## Next Steps

- [Project Structure](project-structure.md) — recommended directory layout
- [Creating an App](../guide/creating-an-app.md) — build a custom app
- [Tutorial](../tutorial/index.md) — build a complete polls app step by step
- [Auth](../contrib/auth.md) — set up WebAuthn passkey authentication
