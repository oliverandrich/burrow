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
    "fmt"
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
    return func(w http.ResponseWriter, r *http.Request, code int, content template.HTML, _ map[string]any) error {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.WriteHeader(code)
        _, err := fmt.Fprintf(w, "<!DOCTYPE html><html><body>%s</body></html>", content)
        return err
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

!!! tip "Use Bootstrap for a real layout"
    The manual layout above is for illustration. In practice, use the `bootstrap` contrib app which provides a full HTML layout with CSS, dark mode, and htmx — see [Bootstrap](../contrib/bootstrap.md). The [Tutorial](../tutorial/index.md) walks through setting this up step by step.

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
