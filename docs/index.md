# Burrow

A modular Go web framework built on [Chi](https://go-chi.io/), [Bun/SQLite](https://bun.uptrace.dev/), and [Templ](https://templ.guide/) templates.

## Features

- **Modular app system** — register self-contained apps with routes, middleware, migrations, and config
- **Pure Go, no CGO** — uses `modernc.org/sqlite` for zero-dependency builds (`CGO_ENABLED=0`)
- **WebAuthn authentication** — passkey-based login with recovery codes and email verification
- **Cookie-based sessions** — encrypted, signed cookies via `gorilla/securecookie`
- **Internationalization** — Accept-Language detection with go-i18n translations
- **Content-hashed static files** — cache-busting URLs computed at startup
- **CSS-agnostic** — bring your own CSS framework and layout templates
- **Convention over configuration** — sensible defaults, override with CLI flags, env vars, or TOML

## Minimal Example

```go
package main

import (
    "context"
    "log"
    "os"

    "codeberg.org/oliverandrich/burrow"
    "codeberg.org/oliverandrich/burrow/contrib/healthcheck"
    "codeberg.org/oliverandrich/burrow/contrib/session"
    "github.com/urfave/cli/v3"
)

func main() {
    srv := burrow.NewServer(
        &session.App{},
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

```bash
go run . --port 8080
curl http://localhost:8080/healthz
# {"database":"ok","status":"ok"}
```

## Quick Links

- [Installation](getting-started/installation.md) — get the module and prerequisites
- [Quick Start](getting-started/quickstart.md) — build a working app in 5 minutes
- [Creating an App](guide/creating-an-app.md) — build a custom app step by step
- [Contrib Apps](contrib/session.md) — use the built-in session, auth, i18n, and more
- [Configuration Reference](reference/configuration.md) — every flag, env var, and TOML key
