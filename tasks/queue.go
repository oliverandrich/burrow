package tasks

import (
	"context"
	"time"
)

// JobHandlerFunc is the signature for job handler functions.
// The context carries a deadline from the worker's shutdown timeout.
// Payload is the raw JSON bytes that were passed to Enqueue.
type JobHandlerFunc func(ctx context.Context, payload []byte) error

// JobOption configures job handler registration.
type JobOption func(*JobConfig)

// JobConfig holds per-handler configuration.
type JobConfig struct {
	MaxRetries int
	Priority   int
}

// WithMaxRetries sets the maximum number of retries for a job type.
func WithMaxRetries(n int) JobOption {
	return func(c *JobConfig) { c.MaxRetries = n }
}

// WithPriority sets the default priority for a job type. Higher values
// mean higher urgency — priority 10 jobs are claimed before priority 0 jobs.
// The default priority is 0.
func WithPriority(n int) JobOption {
	return func(c *JobConfig) { c.Priority = n }
}

// Enqueuer provides job submission and cancellation. Use this interface
// for code that only needs to enqueue jobs, not register handlers.
//
// EnqueueBatch and EnqueueBatchAt insert N jobs of one type atomically:
// either every payload becomes a job or none does. Job IDs are returned in
// input order, an empty payloads slice returns (nil, nil) without touching
// the queue, and each job keeps the independent retry/priority semantics of
// its type — only the insert is batched.
type Enqueuer interface {
	Enqueue(ctx context.Context, typeName string, payload any) (string, error)
	EnqueueAt(ctx context.Context, typeName string, payload any, runAt time.Time) (string, error)
	EnqueueBatch(ctx context.Context, typeName string, payloads []any) ([]string, error)
	EnqueueBatchAt(ctx context.Context, typeName string, payloads []any, runAt time.Time) ([]string, error)
	Dequeue(ctx context.Context, id string) error
}

// Queue provides job handler registration, enqueueing, and cancellation.
// contrib/jobs provides a Den-backed implementation that works on both
// SQLite and Postgres, and optionally against a database separate from
// the application's shared DB (see the jobs-database-dsn flag).
type Queue interface {
	Enqueuer
	Handle(typeName string, fn JobHandlerFunc, opts ...JobOption)
}

// HasJobs is implemented by apps that register background job handlers.
// Called by the Queue implementation during PostConfigure(), after all apps
// have been configured and before workers start. This guarantees that state
// set in Configure() is available when RegisterJobs is invoked.
type HasJobs interface {
	RegisterJobs(q Queue)
}
