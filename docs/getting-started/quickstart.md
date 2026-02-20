# Quick Start

Build a working application with session management, authentication, and a health check endpoint.

## 1. Create the Project

```bash
mkdir myapp && cd myapp
go mod init myapp
go get codeberg.org/oliverandrich/go-webapp-template@latest
```

## 2. Write main.go

```go
package main

import (
    "context"
    "io"
    "log"
    "os"

    "codeberg.org/oliverandrich/go-webapp-template/contrib/auth"
    "codeberg.org/oliverandrich/go-webapp-template/contrib/healthcheck"
    "codeberg.org/oliverandrich/go-webapp-template/contrib/session"
    "codeberg.org/oliverandrich/go-webapp-template/core"
    "github.com/a-h/templ"
    "github.com/urfave/cli/v3"
)

// appLayout wraps page content in a minimal HTML shell.
func appLayout(title string, content templ.Component) templ.Component {
    return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
        _, _ = io.WriteString(w, "<!DOCTYPE html><html><head><title>")
        _, _ = io.WriteString(w, title)
        _, _ = io.WriteString(w, "</title></head><body>")

        // Render navigation from context.
        for _, item := range core.NavItems(ctx) {
            _, _ = io.WriteString(w, `<a href="`)
            _, _ = io.WriteString(w, item.URL)
            _, _ = io.WriteString(w, `">`)
            _, _ = io.WriteString(w, item.Label)
            _, _ = io.WriteString(w, `</a> `)
        }

        if err := content.Render(ctx, w); err != nil {
            return err
        }

        _, _ = io.WriteString(w, "</body></html>")
        return nil
    })
}

func main() {
    // Create the server with apps in dependency order.
    // Session must come before auth (auth depends on session).
    srv := core.NewServer(
        &session.App{},
        auth.New(nil), // nil renderer = API-only, no HTML pages
        &healthcheck.App{},
    )

    srv.SetLayouts(core.Layouts{
        App: appLayout,
    })

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

## 3. Run It

```bash
go run main.go
```

The server starts on `localhost:8080` with:

- SQLite database at `./data/app.db` (auto-created with WAL mode)
- Session cookies (auto-generated keys, logged to stdout)
- Auth routes at `/auth/*` (WebAuthn registration and login)
- Health check at `/healthz`

## 4. Test It

```bash
# Health check
curl http://localhost:8080/healthz
# {"database":"ok","status":"ok"}
```

## 5. Configure It

Override defaults with CLI flags, environment variables, or a TOML config file:

=== "CLI Flags"

    ```bash
    go run main.go --port 3000 --log-level debug --database-dsn ./myapp.db
    ```

=== "Environment Variables"

    ```bash
    PORT=3000 LOG_LEVEL=debug DATABASE_DSN=./myapp.db go run main.go
    ```

See [Configuration](../guide/configuration.md) for the full reference.

## Next Steps

- [Project Structure](project-structure.md) — recommended directory layout
- [Creating an App](../guide/creating-an-app.md) — build a custom app
- [Auth](../contrib/auth.md) — set up WebAuthn passkey authentication
