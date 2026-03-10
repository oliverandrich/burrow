package burrow

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
}

// WithMaxRetries sets the maximum number of retries for a job type.
func WithMaxRetries(n int) JobOption {
	return func(c *JobConfig) { c.MaxRetries = n }
}

// Queue provides job handler registration, enqueueing, and cancellation.
// contrib/jobs provides a SQLite-backed implementation.
type Queue interface {
	Handle(typeName string, fn JobHandlerFunc, opts ...JobOption)
	Enqueue(ctx context.Context, typeName string, payload any) (string, error)
	EnqueueAt(ctx context.Context, typeName string, payload any, runAt time.Time) (string, error)
	Dequeue(ctx context.Context, id string) error
}

// HasJobs is implemented by apps that register background job handlers.
// Called by the Queue implementation during Configure(), before workers start.
type HasJobs interface {
	RegisterJobs(q Queue)
}
