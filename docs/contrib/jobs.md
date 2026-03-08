# Jobs

In-process, SQLite-backed background job queue with a worker pool, retry logic, and admin UI.

**Package:** `codeberg.org/oliverandrich/burrow/contrib/jobs`

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

Register handlers during your app's `Register()` phase — before `Configure()` starts the workers:

```go
func (a *App) Register(cfg *burrow.AppConfig) error {
    jobsApp := cfg.Registry.MustGet("jobs").(*jobs.App)

    jobsApp.Handle("send-welcome-email", func(ctx context.Context, payload string) error {
        var data struct{ Email string }
        json.Unmarshal([]byte(payload), &data)
        return sendWelcomeEmail(ctx, data.Email)
    })

    // With custom max retries (default: 3)
    jobsApp.Handle("process-upload", processUpload, jobs.WithMaxRetries(5))

    return nil
}
```

## Enqueueing Jobs

```go
// Enqueue for immediate processing
job, err := jobsApp.Enqueue(ctx, "send-welcome-email", map[string]string{
    "Email": "alice@example.com",
})

// Schedule for a specific time
job, err := jobsApp.EnqueueAt(ctx, "send-welcome-email", payload, time.Now().Add(time.Hour))
```

The payload can be any value that `json.Marshal` can serialise. The type must be registered via `Handle()` — unknown types return an error.

## Job Lifecycle

Jobs progress through these statuses:

| Status | Description |
|--------|-------------|
| `pending` | Waiting in the queue |
| `running` | Currently being processed by a worker |
| `completed` | Finished successfully |
| `failed` | Failed after all retries exhausted |
| `cancelled` | Manually cancelled via admin UI |

Failed jobs are retried up to `maxRetries` times (default: 3) with no backoff delay. When all retries are exhausted, the job transitions to `failed` with the last error message recorded.

## Admin UI

The jobs app implements `HasAdmin` and provides a ModelAdmin-based admin interface at `/admin/jobs`:

- **List view** with status filter, pagination, and sortable columns
- **Row actions**: Retry (re-queue failed jobs) and Cancel (stop pending/running jobs)
- **Detail view** with pretty-printed JSON payload and error message

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--jobs-workers` | `JOBS_WORKERS` | `2` | Number of concurrent worker goroutines |
| `--jobs-poll-interval` | `JOBS_POLL_INTERVAL` | `1s` | Interval between queue polls |

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
