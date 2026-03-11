# Logging

Burrow uses Go's standard `log/slog` package. The framework logs to `slog.Default()` — it does **not** configure the logger itself. Your application is responsible for setting up the slog handler before starting the server.

## Setting Up the Logger

Configure `slog.SetDefault()` in your `main()` function before calling `srv.Run`:

### Text Output (development)

```go
slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
})))
```

Output:

```
time=2026-03-11T10:00:00.000Z level=INFO msg="starting server" host=localhost port=8080
time=2026-03-11T10:00:01.123Z level=INFO msg="request" method=GET path=/ status=200
```

### JSON Output (production)

```go
slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
})))
```

Output:

```json
{"time":"2026-03-11T10:00:00.000Z","level":"INFO","msg":"starting server","host":"localhost","port":8080}
```

JSON output is useful when shipping logs to aggregators like Loki, Datadog, or Elasticsearch.

### Third-Party Handlers

Any `slog.Handler` works. Popular choices:

- [tint](https://github.com/lmittmann/tint) — colorized terminal output for development
- [slog-multi](https://github.com/samber/slog-multi) — fan-out to multiple handlers

```go
// Example: tint for colorized dev output
slog.SetDefault(slog.New(
    tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelDebug}),
))
```

## What Burrow Logs

The framework logs at these points:

| Event | Level | Message |
|-------|-------|---------|
| Server start | INFO | `starting server` with host, port, TLS mode |
| HTTP requests | INFO | Method, path, status, duration (via [httplog](https://github.com/go-chi/httplog)) |
| Dependency reorder | WARN | When app registration order is adjusted |
| Shutdown errors | ERROR | Failed app shutdown, HTTP server errors |
| Database close errors | ERROR | Failed to close database connection |

Your app code can log using `slog` directly — it shares the same default logger:

```go
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) error {
    // ...
    slog.Info("note created", "note_id", note.ID, "user_id", user.ID)
    // ...
}
```

## Logging in Deployment

All log output goes to **stdout**. Both systemd and Docker capture stdout automatically:

- **systemd** — logs are captured by journald, queryable with `journalctl -u myapp`
- **Docker** — logs are captured by the container runtime, queryable with `docker logs`

No file-based logging configuration is needed. To persist logs beyond the journal or container lifecycle, configure your log aggregator to read from journald or the Docker log driver.
