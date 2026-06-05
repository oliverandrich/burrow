# Jobs

In-process, Den-backed background job queue with a worker pool, retry logic, and admin UI. Runs on SQLite or Postgres, optionally against a database separate from the application's shared DB.

**Package:** `github.com/oliverandrich/burrow/contrib/jobs`

**Depends on:** `admin` (jobs's admin pages share the admin layout and call `{{ template "admin/pagination" }}`; admin transitively requires `staticfiles`, `htmx`, `messages`, `csrf`, `auth`).

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

## Typed Task Definitions (Recommended)

Use `burrow.DefineTask` for type-safe job definitions. It handles JSON marshalling on both sides — producer and consumer agree on the payload type at compile time:

```go
type WelcomeEmailPayload struct {
    Email  string `json:"email"`
    Locale string `json:"locale"`
}

var sendWelcomeEmail = burrow.DefineTask("send-welcome-email",
    func(ctx context.Context, p WelcomeEmailPayload) error {
        return mailer.Send(ctx, p.Email, "Welcome!")
    },
    burrow.WithMaxRetries(5),
    burrow.WithPriority(10), // higher = more urgent
)
```

Register in `RegisterJobs` and enqueue with typed payloads:

```go
func (a *App) RegisterJobs(q burrow.Queue) {
    sendWelcomeEmail.Register(q)
}

// Later, in a handler:
_, err := sendWelcomeEmail.Enqueue(ctx, WelcomeEmailPayload{
    Email: "alice@example.com", Locale: "en",
})
```

### Result-Returning Tasks

For tasks that produce a result, use `burrow.DefineResultTask`. The result is persisted as JSON on the job:

```go
type ReportInput struct {
    UserID string `json:"user_id"`
}
type ReportOutput struct {
    URL string `json:"url"`
}

var generateReport = burrow.DefineResultTask("generate-report",
    func(ctx context.Context, in ReportInput) (ReportOutput, error) {
        url, err := reports.Generate(ctx, in.UserID)
        return ReportOutput{URL: url}, err
    },
)
```

After the job completes, the result is available via the repository's `GetResult` method and visible in the admin detail view.

### Low-Level API

For dynamic job types or when you need full control, use the raw `Queue.Handle` and `Queue.Enqueue` directly:

```go
func (a *App) RegisterJobs(q burrow.Queue) {
    q.Handle("process-upload", func(ctx context.Context, payload []byte) error {
        var params struct{ ImageID string `json:"image_id"` }
        json.Unmarshal(payload, &params)
        return processUpload(ctx, params.ImageID)
    }, burrow.WithMaxRetries(3))
    a.jobs = q
}

// Enqueue:
jobID, err := a.jobs.Enqueue(ctx, "process-upload", map[string]string{"image_id": "123"})
```

The `Enqueuer` interface (`Enqueue`, `EnqueueAt`, `EnqueueBatch`, `EnqueueBatchAt`, `Dequeue`) is available for code that only needs to submit jobs without registering handlers. `Queue` embeds `Enqueuer` and adds `Handle`.

## Enqueueing Jobs

```go
// Enqueue for immediate processing — returns the job ID
jobID, err := sendWelcomeEmail.Enqueue(ctx, WelcomeEmailPayload{Email: "alice@example.com"})

// Schedule for a specific time
jobID, err := sendWelcomeEmail.EnqueueAt(ctx, payload, time.Now().Add(time.Hour))

// Or via the raw Queue interface:
jobID, err := jobsApp.Enqueue(ctx, "send-welcome-email", payload)
```

The payload can be any value that `json.Marshal` can serialise. The type must be registered via `Handle()` or `TaskDefinition.Register()` — unknown types return an error.

### Batch enqueueing

When one event fans out into many jobs of the same type (say, one delivery job per follower), `EnqueueBatch` / `EnqueueBatchAt` insert all of them in a single transaction — one commit instead of N independently committed writes:

```go
var deliverPost = burrow.DefineTask("deliver-post", deliverHandler)

// Typed — payloads is a []DeliveryPayload
ids, err := deliverPost.EnqueueBatch(ctx, payloads)

// Or via the raw Queue interface:
ids, err := jobsApp.EnqueueBatch(ctx, "deliver-post", []any{p1, p2, p3})
```

The insert is all-or-nothing — a payload that fails to marshal rejects the whole batch before anything reaches the database. Job IDs are returned in input order, and an empty slice is a no-op. Each job stays independent after insertion (own retries, own priority from its type), and since all jobs in a batch share one `RunAt`, their relative execution order is not guaranteed.

## Priority

Jobs have a `Priority` field (default 0). Higher values mean higher urgency — a priority-10 job is claimed before a priority-0 job, even if the lower-priority job is older. Within the same priority level, jobs are processed in FIFO order.

Set priority per-type at registration time:

```go
var urgentTask = burrow.DefineTask("urgent-cleanup", handler, burrow.WithPriority(10))
```

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

Once a job has exhausted its `MaxRetries` (default: 3), it transitions to `dead` — a terminal status. The last error message is recorded in `LastError`, and the Go error type is stored in `ErrorClass` (e.g., `*net.OpError`) for monitoring. The `LastAttemptedAt` timestamp records when the last handler execution started.

Jobs that complete successfully store their handler's return value (if any) in the `Result` field as JSON.

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

The jobs app implements `HasAdmin` and provides an admin interface at `/admin/jobs`:

- **List view** with status filter pills, priority column, pagination, and inline action buttons
- **Row actions**: Retry (re-queue dead/failed jobs), Cancel (stop pending/running jobs), Delete
- **Detail view** with pretty-printed JSON payload, result (on success), error message with error class badge, and all timestamps

## Maintenance

The worker pool runs two automatic maintenance tasks every 5 minutes:

**Stale job rescue:** Jobs stuck in `running` for longer than 10 minutes are reset to `pending`. This handles worker crashes where a job was claimed but never completed. The rescue is guarded against concurrent completion — if a worker finishes a job between the stale query and the reset, the job's completed status is preserved.

**Panic recovery:** Worker goroutines recover from panics in job handlers, converting them into failures with a stack trace. The worker stays alive and continues processing other jobs.

**Completed job cleanup:** Jobs in `completed` status older than 24 hours are hard-deleted from the database to prevent unbounded table growth.

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--jobs-workers` | `JOBS_WORKERS` | `2` | Number of concurrent worker goroutines |
| `--jobs-poll-interval` | `JOBS_POLL_INTERVAL` | `1s` | Interval between queue polls |
| `--jobs-retry-base-delay` | `JOBS_RETRY_BASE_DELAY` | `30s` | Base delay for exponential retry backoff |
| `--jobs-database-dsn` | `JOBS_DATABASE_DSN` | (empty) | Database URL for a separate jobs database |

### Separate Database

By default, jobs are stored in the main application database. For applications with high write throughput, you can move the job queue to a dedicated database:

```bash
# Separate SQLite file
./myapp --jobs-database-dsn sqlite:///data/jobs.db

# Or a separate PostgreSQL database
./myapp --jobs-database-dsn postgres://localhost/myapp_jobs
```

This eliminates write contention between the job queue (which produces 3-4 writes per job) and your application's business data.

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
| `HasDocuments` | Registers the `jobs` document type |
| `HasDependencies` | Requires `admin` |
| `Configurable` | Worker count and poll interval flags |
| `HasShutdown` | Stops the worker pool gracefully |
| `HasAdmin` | Admin UI for job management |
| `HasTranslations` | English and German labels for admin UI |
| `HasTemplates` | Admin page templates |
| `HasFuncMap` | Icon and utility template functions |
| `Startable` | Starts the worker pool after full boot, with `TemplateExecutor` for job handlers |
