# tasks — Queue, typed task definitions, result capture

The `burrow/tasks` package holds the type-safe job/task API: the `Queue` and `Enqueuer` interfaces apps program against, the `HasJobs` capability interface apps implement to register their handlers, the generic `TaskDefinition[P]` and `ResultTask[P, R]` wrappers that own JSON marshalling on both ends of the wire, and the result-capture mechanism used by `contrib/jobs`'s worker.

The actual Queue implementation lives in [`contrib/jobs`](../contrib/jobs.md) — Den-backed (SQLite + Postgres), in-process, optionally against a separate database via `--jobs-database-dsn`. Other implementations are possible: alternative contrib apps would satisfy the `Queue` interface and discover handlers the same way.

The burrow root re-exports every name as either an alias or a wrapper, so applications stay on `burrow.DefineTask`, `burrow.Queue`, `burrow.HasJobs`, `burrow.WithMaxRetries`. In files that already import `burrow/tasks` for the typed-task API, the unqualified `tasks.DefineTask[Payload]` spelling reads cleaner.

Full signatures live on [pkg.go.dev](https://pkg.go.dev/github.com/oliverandrich/burrow/tasks). This page is a symbol index.

## Interfaces

| Symbol | Notes |
|---|---|
| `Queue` | Embeds `Enqueuer` plus `Handle(typeName, fn, opts...)` for handler registration |
| `Enqueuer` | `Enqueue(ctx, typeName, payload)`, `EnqueueAt(ctx, typeName, payload, runAt)`, `EnqueueBatch(ctx, typeName, payloads)`, `EnqueueBatchAt(ctx, typeName, payloads, runAt)`, `Dequeue(ctx, id)` — for code that only submits |
| `HasJobs` | App capability — `RegisterJobs(q Queue)` called once during PostConfigure |
| `JobHandlerFunc` | `func(ctx, []byte) error` — raw handler signature |

## Typed task definitions

| Symbol | Notes |
|---|---|
| `TaskDefinition[P any]` | Generic typed task; ships a `Register` method to wire it into a Queue and `Enqueue` / `EnqueueAt` / `EnqueueBatch` / `EnqueueBatchAt` methods that marshal the payload(s) |
| `DefineTask[P](name, handler, opts...)` | Constructor — handler is `func(ctx, P) error` |
| `ResultTask[P, R any]` | Like `TaskDefinition` but the handler returns a value that the worker stores via `ResultCapture` |
| `DefineResultTask[P, R](name, handler, opts...)` | Constructor — handler is `func(ctx, P) (R, error)` |

## Result capture

| Symbol | Notes |
|---|---|
| `ResultCapture` | Holds the data the worker stores for a completed job (`contrib/jobs` reads this from the context after the handler returns) |
| `WithResultCapture(ctx, rc)` | Inject a ResultCapture into the worker's per-job context |
| `CaptureResult(ctx, data)` | Helper called by `ResultTask.handler` to write the result bytes |

## Per-handler options

| Symbol | Notes |
|---|---|
| `JobConfig` | `MaxRetries`, `Priority` — populated by options |
| `JobOption` | Functional option type |
| `WithMaxRetries(n)` | Override the default retry count for one task type |
| `WithPriority(n)` | Higher priority = claimed first; default 0 |

See the [Jobs guide](../contrib/jobs.md) for full usage including worker configuration, scheduling, and the admin UI.
