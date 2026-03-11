# Jobs

In-process, SQLite-backed background job queue with a worker pool, retry logic, and admin UI.

**Package:** `github.com/oliverandrich/burrow/contrib/jobs`

**Depends on:** none (optional: `admin` for admin panel UI)

## Setup

```go
jobsApp := jobs.New()

srv := burrow.NewServer(
    session.New(),
    jobsApp,
    admin.New(),
    // ... other apps
)
```

## Registering Job Handlers

Register handlers during your app's `Register()` phase — before `Configure()` starts the workers. The handler type is `burrow.JobHandlerFunc`:

```go
// Defined in the burrow root package:
// type JobHandlerFunc func(ctx context.Context, payload []byte) error
```

```go
func (a *App) Register(cfg *burrow.AppConfig) error {
    app, _ := cfg.Registry.Get("jobs")
    jobsApp := app.(*jobs.App)

    jobsApp.Handle("send-welcome-email", func(ctx context.Context, payload []byte) error {
        var data struct{ Email string }
        if err := json.Unmarshal(payload, &data); err != nil {
            return fmt.Errorf("invalid payload: %w", err)
        }
        return sendWelcomeEmail(ctx, data.Email)
    })

    // With custom max retries (default: 3)
    jobsApp.Handle("process-upload", processUpload, burrow.WithMaxRetries(5))

    return nil
}
```

### Accessing Job Data in Handlers

The handler receives the raw JSON `payload` as `[]byte` — the same data you passed when enqueueing, marshaled to JSON:

```go
jobsApp.Handle("resize-image", func(ctx context.Context, payload []byte) error {
    var params struct {
        ImageID int64  `json:"image_id"`
        Width   int    `json:"width"`
    }
    if err := json.Unmarshal(payload, &params); err != nil {
        return fmt.Errorf("invalid payload: %w", err)
    }

    return resizeImage(ctx, params.ImageID, params.Width)
})

## Enqueueing Jobs

```go
// Enqueue for immediate processing — returns the job ID as a string
jobID, err := jobsApp.Enqueue(ctx, "send-welcome-email", map[string]string{
    "Email": "alice@example.com",
})

// Schedule for a specific time
jobID, err := jobsApp.EnqueueAt(ctx, "send-welcome-email", payload, time.Now().Add(time.Hour))
```

The payload can be any value that `json.Marshal` can serialise. The type must be registered via `Handle()` — unknown types return an error.

## Job Lifecycle

Jobs progress through these statuses:

| Status | Description |
|--------|-------------|
| `pending` | Waiting in the queue |
| `running` | Currently being processed by a worker |
| `completed` | Finished successfully |
| `failed` | Failed, will be retried |
| `dead` | Terminal — all retries exhausted or manually cancelled |

## Retry & Backoff

When a handler returns an error, the job is marked `failed` and scheduled for retry with **exponential backoff**:

```
delay = base_delay * 2^(attempt-1)
```

With the default base delay of 30 seconds:

| Attempt | Delay |
|---------|-------|
| 1 | 30s |
| 2 | 1m |
| 3 | 2m |
| 4 | 4m |
| 5 | 8m |

Once a job has exhausted its `MaxRetries` (default: 3), it transitions to `dead` — a terminal status. The last error message is recorded in `LastError`.

Jobs can also reach `dead` by being manually cancelled via the admin UI.

## Admin UI

The jobs app implements `HasAdmin` and provides a ModelAdmin-based admin interface at `/admin/jobs`:

- **List view** with status filter, pagination, and sortable columns
- **Row actions**: Retry (re-queue dead jobs) and Cancel (stop pending/running jobs)
- **Detail view** with pretty-printed JSON payload and error message

## Maintenance

The worker pool runs two automatic maintenance tasks every 5 minutes:

**Stale job rescue:** Jobs stuck in `running` for longer than 10 minutes are reset to `pending`. This handles worker crashes or panics where a job was claimed but never completed.

**Completed job cleanup:** Jobs in `completed` status older than 24 hours are hard-deleted from the database to prevent unbounded table growth.

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--jobs-workers` | `JOBS_WORKERS` | `2` | Number of concurrent worker goroutines |
| `--jobs-poll-interval` | `JOBS_POLL_INTERVAL` | `1s` | Interval between queue polls |
| `--jobs-retry-base-delay` | `JOBS_RETRY_BASE_DELAY` | `30s` | Base delay for exponential retry backoff |

## Graceful Shutdown

The jobs app implements `HasShutdown`. When the server shuts down:

1. The worker pool stops accepting new jobs
2. In-flight jobs are allowed to complete
3. The `Done()` channel is closed once all workers have finished

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `Migratable` | Creates the `jobs` table |
| `Configurable` | Worker count and poll interval flags |
| `HasShutdown` | Stops the worker pool gracefully |
| `HasAdmin` | Admin UI for job management |
| `HasTranslations` | English and German labels for admin UI |
| `HasTemplates` | Admin page templates |
| `HasFuncMap` | Icon and utility template functions |
