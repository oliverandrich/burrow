package burrow

import (
	"context"

	"github.com/oliverandrich/burrow/tasks"
)

// WithMaxRetries sets the maximum number of retries for a job type. Wrapper
// around [tasks.WithMaxRetries].
func WithMaxRetries(n int) JobOption { return tasks.WithMaxRetries(n) }

// WithPriority sets the default priority for a job type. Wrapper around
// [tasks.WithPriority].
func WithPriority(n int) JobOption { return tasks.WithPriority(n) }

// DefineTask wires up a generic typed task definition. Wrapper around
// [tasks.DefineTask].
func DefineTask[P any](name string, handler func(context.Context, P) error, opts ...JobOption) *TaskDefinition[P] {
	return tasks.DefineTask(name, handler, opts...)
}

// DefineResultTask wires up a generic typed task that captures a result.
// Wrapper around [tasks.DefineResultTask].
func DefineResultTask[P, R any](name string, handler func(context.Context, P) (R, error), opts ...JobOption) *ResultTask[P, R] {
	return tasks.DefineResultTask(name, handler, opts...)
}

// WithResultCapture stores a ResultCapture in the context. Wrapper around
// [tasks.WithResultCapture].
func WithResultCapture(ctx context.Context, rc *ResultCapture) context.Context {
	return tasks.WithResultCapture(ctx, rc)
}

// CaptureResult stores result data in the context-bound capture. Wrapper
// around [tasks.CaptureResult].
func CaptureResult(ctx context.Context, data []byte) { tasks.CaptureResult(ctx, data) }
