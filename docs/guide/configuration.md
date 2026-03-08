# Configuration

The framework uses a three-tier configuration system. Values are resolved in this priority order:

1. **CLI flags** (highest priority)
2. **Environment variables**
3. **TOML config file**
4. **Default values** (lowest priority)

## Using CLI Flags

```bash
go run main.go --port 3000 --database-dsn ./myapp.db
```

## Using Environment Variables

```bash
PORT=3000 DATABASE_DSN=./myapp.db go run main.go
```

## Using a TOML Config File

To enable TOML configuration, pass a config source to `srv.Flags()`:

```go
import "github.com/urfave/cli/v3"

// Load TOML file as a config source.
configSource := func(key string) cli.ValueSource {
    return cli.TOMLSourceFromFile("config.toml", key)
}

cmd := &cli.Command{
    Flags:  srv.Flags(configSource),
    Action: srv.Run,
}
```

Then create `config.toml`:

```toml
[server]
host = "0.0.0.0"
port = 3000
base_url = "https://myapp.example.com"
max_body_size = 2

[database]
dsn = "./data/production.db"

[tls]
mode = "acme"
email = "admin@example.com"
```

## Adding Custom Flags

Apps implement the `Configurable` interface to add their own flags:

```go
func (a *App) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
    return []cli.Flag{
        &cli.StringFlag{
            Name:    "notes-page-size",
            Value:   "20",
            Usage:   "Number of notes per page",
            Sources: cli.EnvVars("NOTES_PAGE_SIZE"),
        },
    }
}

func (a *App) Configure(cmd *cli.Command) error {
    a.pageSize = int(cmd.Int("notes-page-size"))
    return nil
}
```

Custom flags are automatically merged into the CLI when you call `srv.Flags()`.

See the [TLS guide](tls.md) for HTTPS configuration and the [Configuration Reference](../reference/configuration.md) for the complete flag table.
