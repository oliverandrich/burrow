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

The recommended way to register handlers is via the `burrow.HasJobs` interface. Implement `RegisterJobs(q burrow.Queue)` on your app — the jobs app discovers all `HasJobs` implementors automatically during its `PostConfigure()` phase:

```go
// Defined in the burrow root package:
// type JobHandlerFunc func(ctx context.Context, payload []byte) error
```

```go
func (a *App) RegisterJobs(q burrow.Queue) {
    q.Handle("send-welcome-email", func(ctx context.Context, payload []byte) error {
        var data struct{ Email string }
        if err := json.Unmarshal(payload, &data); err != nil {
            return fmt.Errorf("invalid payload: %w", err)
        }
        return sendWelcomeEmail(ctx, data.Email)
    })

    // With custom max retries (default: 3)
    q.Handle("process-upload", a.processUpload, burrow.WithMaxRetries(5))

    // Save the queue reference for enqueueing later
    a.jobs = q
}
```

Because `PostConfigure()` runs after all `Configure()` calls, your app can safely use state set in `Configure()` inside `RegisterJobs` (e.g., services, config values, clients).

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

## Rendering Templates in Job Handlers

The jobs app receives the `TemplateExecutor` from the server during the [`Startable`](../reference/interfaces.md#startable) lifecycle phase and injects it into every job handler's context at execution time. This means you can use `burrow.RenderFragment` directly in background jobs — no manual setup required.

```go
func (a *App) handleSendEmail(ctx context.Context, payload []byte) error {
    var data struct {
        UserID int64  `json:"user_id"`
        Locale string `json:"locale"`
    }
    if err := json.Unmarshal(payload, &data); err != nil {
        return fmt.Errorf("invalid payload: %w", err)
    }

    user, err := a.repo.GetUser(ctx, data.UserID)
    if err != nil {
        return err
    }

    // Set locale for i18n template functions.
    ctx = a.withLocale(ctx, data.Locale)

    // Render the email body using a template — works because
    // the TemplateExecutor is already in the context.
    body, err := burrow.RenderFragment(ctx, "emails/welcome", map[string]any{
        "User": user,
    })
    if err != nil {
        return fmt.Errorf("render email: %w", err)
    }

    return a.mailer.Send(user.Email, "Welcome!", string(body))
}
```

!!! tip "i18n in job handlers"
    Template functions like `t`, `tData`, and `tPlural` depend on the locale in the context. When rendering localized templates in jobs, save the `WithLocale` function from `AppConfig` during `Register` and use it to set the locale before calling `RenderFragment`.

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
| `--jobs-database` | `JOBS_DATABASE` | (empty) | Path to a separate SQLite database for the job queue |

### Separate Database

By default, jobs are stored in the main application database. For applications with high write throughput, you can move the job queue to a dedicated SQLite file:

```bash
./myapp --jobs-database data/jobs.db
```

This eliminates write contention between the job queue (which produces 3-4 writes per job) and your application's business data. The separate database gets the same PRAGMA configuration (WAL mode, foreign keys, busy timeout, etc.) as the main database.

When no value is set, the jobs app uses the shared database — no configuration change is needed.

## Graceful Shutdown

The jobs app implements `HasShutdown`. When the server shuts down:

1. The worker pool stops accepting new jobs
2. In-flight jobs are allowed to complete
3. The `Done()` channel is closed once all workers have finished

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()` |
| `Migratable` | Creates the `jobs` table |
| `Configurable` | Worker count and poll interval flags |
| `HasShutdown` | Stops the worker pool gracefully |
| `HasAdmin` | Admin UI for job management |
| `HasTranslations` | English and German labels for admin UI |
| `HasTemplates` | Admin page templates |
| `HasFuncMap` | Icon and utility template functions |
| `Startable` | Starts the worker pool after full boot, with `TemplateExecutor` for job handlers |
