# Quick Start

Build a working application with session management, a health check endpoint, and a custom homepage.

## 1. Create the Project

```bash
mkdir myapp && cd myapp
go mod init myapp
go get codeberg.org/oliverandrich/burrow@latest
```

## 2. Write main.go

```go
package main

import (
    "context"
    "html/template"
    "log"
    "net/http"
    "os"

    "codeberg.org/oliverandrich/burrow"
    "codeberg.org/oliverandrich/burrow/contrib/healthcheck"
    "codeberg.org/oliverandrich/burrow/contrib/session"
    "github.com/urfave/cli/v3"
)

// appLayout wraps page content in a minimal HTML shell.
func appLayout() burrow.LayoutFunc {
    return func(w http.ResponseWriter, r *http.Request, code int, content template.HTML, data map[string]any) error {
        title, _ := data["Title"].(string)

        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.WriteHeader(code)

        _, _ = w.Write([]byte("<!DOCTYPE html><html><head><title>"))
        _, _ = w.Write([]byte(title))
        _, _ = w.Write([]byte("</title></head><body>"))

        // Render navigation from context.
        for _, item := range burrow.NavItems(r.Context()) {
            _, _ = w.Write([]byte(`<a href="`))
            _, _ = w.Write([]byte(item.URL))
            _, _ = w.Write([]byte(`">`))
            _, _ = w.Write([]byte(item.Label))
            _, _ = w.Write([]byte(`</a> `))
        }

        _, _ = w.Write([]byte(content))
        _, _ = w.Write([]byte("</body></html>"))
        return nil
    }
}

func main() {
    srv := burrow.NewServer(
        session.New(),
        healthcheck.New(),
    )

    srv.SetLayout(appLayout())

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

!!! tip "Use Bootstrap for a proper layout"
    The manual layout above is for illustration only. In practice, use the `bootstrap` contrib app which provides a full layout with CSS, icons, dark mode, and htmx — see [Bootstrap](../contrib/bootstrap.md).

## 3. Run It

```bash
go run main.go
```

The server starts on `localhost:8080` with:

- SQLite database at `app.db` in the working directory (auto-created with WAL mode)
- Session cookies (auto-generated keys, logged to stdout)
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
