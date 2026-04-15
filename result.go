package burrow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type resultCaptureKey struct{}

// ResultCapture collects a handler's return value so the worker can persist it.
// The worker injects a ResultCapture into the handler context; result-aware
// handlers (via ResultTask) write their marshalled result into it.
type ResultCapture struct {
	data string
}

// Result returns the captured JSON result, or empty string if nothing was captured.
func (rc *ResultCapture) Result() string { return rc.data }

// WithResultCapture returns a context carrying the given ResultCapture.
func WithResultCapture(ctx context.Context, rc *ResultCapture) context.Context {
	return context.WithValue(ctx, resultCaptureKey{}, rc)
}

// CaptureResult writes a result into the context's ResultCapture.
// It is a no-op if no capture is present in the context.
func CaptureResult(ctx context.Context, data []byte) {
	if rc, ok := ctx.Value(resultCaptureKey{}).(*ResultCapture); ok {
		rc.data = string(data)
	}
}

// ResultTask is a type-safe wrapper for a job type whose handler returns
// both a result value and an error. It handles JSON marshalling of both
// payload and result automatically. The result is communicated back to the
// worker via the context's ResultCapture.
//
// Create with DefineResultTask, wire with Register, enqueue with Enqueue:
//
//	var compute = burrow.DefineResultTask[Input, Output]("compute",
//	    func(ctx context.Context, in Input) (Output, error) { ... },
//	)
//
//	func (a *App) RegisterJobs(q burrow.Queue) { compute.Register(q) }
//
//	compute.Enqueue(ctx, Input{...})
type ResultTask[P, R any] struct {
	name    string
	handler func(context.Context, P) (R, error)
	opts    []JobOption
	queue   Enqueuer
}

// DefineResultTask creates a typed task definition with a result-returning
// handler. Call Register in your RegisterJobs method to wire it to a Queue.
func DefineResultTask[P, R any](name string, handler func(context.Context, P) (R, error), opts ...JobOption) *ResultTask[P, R] {
	return &ResultTask[P, R]{
		name:    name,
		handler: handler,
		opts:    opts,
	}
}

// Name returns the task type name.
func (t *ResultTask[P, R]) Name() string { return t.name }

// Register wires the task to a Queue. It registers a JobHandlerFunc that
// unmarshals the payload, calls the typed handler, marshals the result
// into the context's ResultCapture, and returns the error.
func (t *ResultTask[P, R]) Register(q Queue) {
	q.Handle(t.name, func(ctx context.Context, payload []byte) error {
		var p P
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("unmarshal %s payload: %w", t.name, err)
		}
		result, err := t.handler(ctx, p)
		if err != nil {
			return err
		}
		data, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return fmt.Errorf("marshal %s result: %w", t.name, marshalErr)
		}
		CaptureResult(ctx, data)
		return nil
	}, t.opts...)
	t.queue = q
}

// Enqueue marshals the payload and enqueues the task for immediate processing.
// Panics if called before Register.
func (t *ResultTask[P, R]) Enqueue(ctx context.Context, payload P) (string, error) {
	t.mustBeRegistered()
	return t.queue.Enqueue(ctx, t.name, payload)
}

// EnqueueAt marshals the payload and enqueues the task for processing at
// the given time. Panics if called before Register.
func (t *ResultTask[P, R]) EnqueueAt(ctx context.Context, payload P, runAt time.Time) (string, error) {
	t.mustBeRegistered()
	return t.queue.EnqueueAt(ctx, t.name, payload, runAt)
}

func (t *ResultTask[P, R]) mustBeRegistered() {
	if t.queue == nil {
		panic(fmt.Sprintf("burrow: ResultTask %q used before Register was called", t.name))
	}
}
