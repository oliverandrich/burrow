package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// TaskDefinition is a type-safe wrapper for a job type. It handles JSON
// marshalling/unmarshalling of the payload automatically, ensuring that
// the enqueue site and handler always agree on the payload type at
// compile time.
//
// Create a TaskDefinition with DefineTask, wire it to a Queue with
// Register (typically in your RegisterJobs method), then use Enqueue
// to submit work:
//
//	var sendEmail = tasks.DefineTask[EmailPayload]("send-email",
//	    func(ctx context.Context, p EmailPayload) error { ... },
//	    tasks.WithMaxRetries(5),
//	)
//
//	func (a *App) RegisterJobs(q tasks.Queue) { sendEmail.Register(q) }
//
//	sendEmail.Enqueue(ctx, EmailPayload{To: "x@y.com"})
type TaskDefinition[P any] struct {
	name    string
	handler func(context.Context, P) error
	opts    []JobOption
	queue   Enqueuer
}

// DefineTask creates a typed task definition. Call Register in your
// RegisterJobs method to wire it to a Queue before enqueueing work.
func DefineTask[P any](name string, handler func(context.Context, P) error, opts ...JobOption) *TaskDefinition[P] {
	return &TaskDefinition[P]{
		name:    name,
		handler: handler,
		opts:    opts,
	}
}

// Name returns the task type name.
func (t *TaskDefinition[P]) Name() string { return t.name }

// Register wires the task to a Queue. It registers a JobHandlerFunc that
// automatically unmarshals the JSON payload into P before calling the
// typed handler, and stores the queue reference for Enqueue/EnqueueAt.
func (t *TaskDefinition[P]) Register(q Queue) {
	q.Handle(t.name, func(ctx context.Context, payload []byte) error {
		var p P
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("unmarshal %s payload: %w", t.name, err)
		}
		return t.handler(ctx, p)
	}, t.opts...)
	t.queue = q
}

// Enqueue marshals the payload and enqueues the task for immediate processing.
// Panics if called before Register.
func (t *TaskDefinition[P]) Enqueue(ctx context.Context, payload P) (string, error) {
	t.mustBeRegistered()
	return t.queue.Enqueue(ctx, t.name, payload)
}

// EnqueueAt marshals the payload and enqueues the task for processing at
// the given time. Panics if called before Register.
func (t *TaskDefinition[P]) EnqueueAt(ctx context.Context, payload P, runAt time.Time) (string, error) {
	t.mustBeRegistered()
	return t.queue.EnqueueAt(ctx, t.name, payload, runAt)
}

func (t *TaskDefinition[P]) mustBeRegistered() {
	if t.queue == nil {
		panic(fmt.Sprintf("burrow: TaskDefinition %q used before Register was called", t.name))
	}
}
