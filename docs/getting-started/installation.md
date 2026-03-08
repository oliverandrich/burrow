# Installation

## Requirements

- **Go 1.25+**
- **CGO_ENABLED=0** — the framework uses pure-Go SQLite (`modernc.org/sqlite`), no C compiler needed

## Install the Module

```bash
go get github.com/oliverandrich/burrow@latest
```

This pulls the `burrow` package and all `contrib` apps. Import only what you need:

```go
import (
    "github.com/oliverandrich/burrow"
    "github.com/oliverandrich/burrow/contrib/session"
    "github.com/oliverandrich/burrow/contrib/auth"
    "github.com/oliverandrich/burrow/contrib/healthcheck"
)
```

## Key Dependencies

The framework builds on these libraries — you'll interact with them when building apps:

| Library | Purpose |
|---------|---------|
| [Chi v5](https://go-chi.io/) | HTTP router and middleware (stdlib-compatible) |
| [Bun](https://bun.uptrace.dev/) | ORM for SQLite (queries, models, migrations) |
| [`html/template`](https://pkg.go.dev/html/template) | Go's standard template engine with auto-escaping |
| [urfave/cli](https://cli.urfave.org/) | CLI flags, env vars, subcommands |

## Verify

Create a minimal `main.go` to check that everything works:

```go
package main

import (
    "github.com/oliverandrich/burrow"
    "github.com/oliverandrich/burrow/contrib/healthcheck"
    "github.com/urfave/cli/v3"
)

func main() {
    srv := burrow.NewServer(healthcheck.New())
    cmd := &cli.Command{
        Flags:  srv.Flags(nil),
        Action: srv.Run,
    }
    cmd.Run(nil, nil)
}
```

```bash
go run main.go
# time=... level=INFO msg="starting server" host=localhost port=8080 ...
# Visit http://localhost:8080/healthz → "ok"
```

See [Quick Start](quickstart.md) for a more complete example.
