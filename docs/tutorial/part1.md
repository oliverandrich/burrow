# Part 1: Setup & First View

In this first part you'll create a Go module, set up a minimal Burrow server, and serve a "Hello, Polls!" homepage.

**Source code:** [`tutorial/step01/`](https://codeberg.org/oliverandrich/burrow/src/branch/main/tutorial/step01)

## Create the Project

```bash
mkdir polls && cd polls
go mod init polls
go get codeberg.org/oliverandrich/burrow@latest
```

After writing the code below, run `go mod tidy` to fetch all transitive dependencies before building.

## Write the Server

Create `main.go`:

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"

    "codeberg.org/oliverandrich/burrow"
    "github.com/go-chi/chi/v5"
    "github.com/urfave/cli/v3"
)

func main() {
    srv := burrow.NewServer(
        &homepageApp{},
    )

    cmd := &cli.Command{
        Name:    "polls",
        Usage:   "Polls tutorial application",
        Version: "0.1.0",
        Flags:   srv.Flags(nil),
        Action:  srv.Run,
    }

    if err := cmd.Run(context.Background(), os.Args); err != nil {
        log.Fatal(err)
    }
}
```

### The Homepage App

Add the following code to the same `main.go` file, below the `main()` function.

Every Burrow application is composed of **apps**. Each app implements `burrow.App`:

```go
type App interface {
    Name() string
    Register(cfg *AppConfig) error
}
```

Our homepage app also implements `HasRoutes` to register an HTTP endpoint:

```go
type homepageApp struct{}

func (a *homepageApp) Name() string                        { return "homepage" }
func (a *homepageApp) Register(_ *burrow.AppConfig) error  { return nil }
func (a *homepageApp) Routes(r chi.Router) {
    r.Get("/", burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
        return burrow.Text(w, http.StatusOK, "Hello, Polls!")
    }))
}
```

A few things to note:

- **`burrow.HandlerFunc`** has the signature `func(w http.ResponseWriter, r *http.Request) error` — just like the standard library, but with an error return. This lets you propagate errors instead of handling them in every handler.
- **`burrow.Handle()`** wraps a `HandlerFunc` into a standard `http.HandlerFunc`. If the handler returns an `*HTTPError`, it sends the appropriate status code and message.
- **`burrow.Text()`** is a helper that writes a plain text response.

## Run It

```bash
go run .
```

You should see log output indicating the server has started. Burrow creates an `app.db` SQLite database in the working directory. Open `http://localhost:8080` in your browser — you'll see "Hello, Polls!".

## What Happens at Boot

When `srv.Run` is called, Burrow follows this sequence:

1. Parse configuration from CLI flags, environment variables, and TOML files
2. Open the SQLite database (WAL mode, foreign keys enabled)
3. Run migrations for all `Migratable` apps
4. Call `Register()` on each app with the shared `AppConfig`
5. Build the global template set from all `HasTemplates` apps
6. Set up the Chi router with core middleware
7. Apply middleware from all `HasMiddleware` apps
8. Register routes from all `HasRoutes` apps
9. Start the HTTP server with graceful shutdown

## Built-in CLI Flags

Because Burrow uses [urfave/cli](https://github.com/urfave/cli), your server gets flags for free:

```bash
go run . --help
go run . --host 0.0.0.0 --port 3000
```

Common flags include `--host`, `--port`, `--database-dsn`, and `--log-level`.

## Next

In [Part 2](part2.md), you'll add a database, define models for questions and choices, and write your first migration.
