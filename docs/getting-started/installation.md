# Installation

## Requirements

- **Go 1.25+**
- **CGO_ENABLED=0** — the framework uses pure-Go SQLite (`modernc.org/sqlite`), no C compiler needed

## Install the Module

```bash
go get codeberg.org/oliverandrich/burrow@latest
```

This pulls the `burrow` package and all `contrib` apps. Import only what you need:

```go
import (
    "codeberg.org/oliverandrich/burrow"
    "codeberg.org/oliverandrich/burrow/contrib/session"
    "codeberg.org/oliverandrich/burrow/contrib/auth"
    "codeberg.org/oliverandrich/burrow/contrib/healthcheck"
)
```

## Key Dependencies

The framework builds on these libraries — you'll interact with them when building apps:

| Library | Purpose |
|---------|---------|
| [Echo v5](https://echo.labstack.com/) | HTTP router, middleware, context |
| [Bun](https://bun.uptrace.dev/) | ORM for SQLite (queries, models, migrations) |
| [Templ](https://templ.guide/) | Type-safe HTML templates |
| [urfave/cli](https://cli.urfave.org/) | CLI flags, env vars, subcommands |

## Verify

Create a `main.go` and run it:

```bash
go run main.go
# time=... level=INFO msg="starting server" host=localhost port=8080 ...
```

See [Quick Start](quickstart.md) for the full walkthrough.
