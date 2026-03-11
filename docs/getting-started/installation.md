# Installation

## Requirements

- **Go 1.25+**
- **CGO_ENABLED=0** — the framework uses pure-Go SQLite (`modernc.org/sqlite`), no C compiler needed

## Create a Project

```bash
mkdir myapp && cd myapp
go mod init myapp
go get github.com/oliverandrich/burrow@latest
```

This creates a Go module and pulls the `burrow` package with all `contrib` apps. Import only what you need:

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
    "context"
    "log"
    "os"

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
    if err := cmd.Run(context.Background(), os.Args); err != nil {
        log.Fatal(err)
    }
}
```

```bash
go mod tidy
go run main.go
# time=... level=INFO msg="starting server" host=localhost port=8080 ...
# Visit http://localhost:8080/healthz → "ok"
```

See [Quick Start](quickstart.md) for a more complete example.
