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

Register handlers during your app's `Register()` phase — before `Configure()` starts the workers:

```go
func (a *App) Register(cfg *burrow.AppConfig) error {
    app, _ := cfg.Registry.Get("jobs")
    jobsApp := app.(*jobs.App)

    jobsApp.Handle("send-welcome-email", func(ctx context.Context, job *jobs.Job) error {
        var data struct{ Email string }
        json.Unmarshal([]byte(job.Payload), &data)
        return sendWelcomeEmail(ctx, data.Email)
    })

    // With custom max retries (default: 3)
    jobsApp.Handle("process-upload", processUpload, jobs.WithMaxRetries(5))

    return nil
}
```

### Accessing Job Data in Handlers

The handler receives the full `*jobs.Job` struct. The `Payload` field contains the JSON-encoded data you passed when enqueueing:

```go
jobsApp.Handle("resize-image", func(ctx context.Context, job *jobs.Job) error {
    var params struct {
        ImageID int64  `json:"image_id"`
        Width   int    `json:"width"`
    }
    if err := json.Unmarshal([]byte(job.Payload), &params); err != nil {
        return fmt.Errorf("invalid payload: %w", err)
    }

    log.Printf("attempt %d for job %d", job.Attempts, job.ID)
    return resizeImage(ctx, params.ImageID, params.Width)
})
```

Available fields on `*jobs.Job`:

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `int64` | Unique job identifier |
| `Type` | `string` | Registered job type name |
| `Payload` | `string` | JSON-encoded payload |
| `Status` | `JobStatus` | Current status |
| `Attempts` | `int` | Number of attempts so far |
| `MaxRetries` | `int` | Maximum retry count |
| `LastError` | `string` | Error message from last failure |
| `CreatedAt` | `time.Time` | When the job was created |

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
| `failed` | Failed, will be retried |
| `dead` | Terminal — all retries exhausted or manually cancelled |

## Retry & Backoff

When a handler returns an error, the job is marked `failed` and scheduled for retry with **exponential backoff**:

```
delay = 2^attempts seconds
```

| Attempt | Delay |
|---------|-------|
| 1 | 2s |
| 2 | 4s |
| 3 | 8s |
| 4 | 16s |

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
