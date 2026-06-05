package tasks_test

import (
	"context"
	"testing"
	"time"

	"github.com/oliverandrich/burrow/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type resultPayload struct {
	Input string `json:"input"`
}

type resultOutput struct {
	Output string `json:"output"`
}

func TestResultTask_RegisterAndEnqueue(t *testing.T) {
	var received resultPayload
	task := tasks.DefineResultTask("compute", func(_ context.Context, p resultPayload) (resultOutput, error) {
		received = p
		return resultOutput{Output: "done-" + p.Input}, nil
	})

	q := newMockQueue()
	task.Register(q)

	// Verify handler was registered.
	require.Contains(t, q.handlers, "compute")

	// Enqueue a typed payload.
	id, err := task.Enqueue(context.Background(), resultPayload{Input: "hello"})
	require.NoError(t, err)
	assert.Equal(t, "job-123", id)

	// Simulate worker: inject ResultCapture, call handler, read result.
	capture := &tasks.ResultCapture{}
	ctx := tasks.WithResultCapture(context.Background(), capture)

	handler := q.handlers["compute"]
	err = handler(ctx, []byte(`{"input":"hello"}`))
	require.NoError(t, err)
	assert.Equal(t, "hello", received.Input)

	// The result should have been captured.
	assert.JSONEq(t, `{"output":"done-hello"}`, capture.Result())
}

func TestResultTask_EnqueueAt(t *testing.T) {
	task := tasks.DefineResultTask("delayed", func(_ context.Context, _ resultPayload) (resultOutput, error) {
		return resultOutput{}, nil
	})

	q := newMockQueue()
	task.Register(q)

	runAt := time.Now().Add(time.Hour)
	id, err := task.EnqueueAt(context.Background(), resultPayload{Input: "later"}, runAt)
	require.NoError(t, err)
	assert.Equal(t, "job-456", id)
}

func TestResultTask_EnqueueBatch(t *testing.T) {
	task := tasks.DefineResultTask("batch-compute", func(_ context.Context, _ resultPayload) (resultOutput, error) {
		return resultOutput{}, nil
	})

	q := newMockQueue()
	task.Register(q)

	payloads := []resultPayload{{Input: "a"}, {Input: "b"}}
	ids, err := task.EnqueueBatch(context.Background(), payloads)
	require.NoError(t, err)
	assert.Equal(t, []string{"job-b1", "job-b2"}, ids)
	assert.Equal(t, "batch-compute", q.batchType)
	assert.Equal(t, []any{payloads[0], payloads[1]}, q.batchPayloads)
}

func TestResultTask_EnqueueBatchAt(t *testing.T) {
	task := tasks.DefineResultTask("batch-later", func(_ context.Context, _ resultPayload) (resultOutput, error) {
		return resultOutput{}, nil
	})

	q := newMockQueue()
	task.Register(q)

	runAt := time.Now().Add(time.Hour)
	ids, err := task.EnqueueBatchAt(context.Background(), []resultPayload{{Input: "later"}}, runAt)
	require.NoError(t, err)
	assert.Equal(t, []string{"job-b1"}, ids)
	assert.Equal(t, runAt, q.batchRunAt)
}

func TestResultTask_EnqueueBatchBeforeRegister(t *testing.T) {
	task := tasks.DefineResultTask("unregistered", func(_ context.Context, _ resultPayload) (resultOutput, error) {
		return resultOutput{}, nil
	})

	assert.PanicsWithValue(t,
		`burrow: ResultTask "unregistered" used before Register was called`,
		func() {
			_, _ = task.EnqueueBatch(context.Background(), []resultPayload{{}})
		},
	)
}

func TestResultTask_Name(t *testing.T) {
	task := tasks.DefineResultTask("my-task", func(_ context.Context, _ resultPayload) (resultOutput, error) {
		return resultOutput{}, nil
	})
	assert.Equal(t, "my-task", task.Name())
}

func TestResultTask_EnqueueBeforeRegister(t *testing.T) {
	task := tasks.DefineResultTask("unregistered", func(_ context.Context, _ resultPayload) (resultOutput, error) {
		return resultOutput{}, nil
	})

	assert.PanicsWithValue(t,
		`burrow: ResultTask "unregistered" used before Register was called`,
		func() {
			_, _ = task.Enqueue(context.Background(), resultPayload{})
		},
	)
}

func TestResultTask_HandlerUnmarshalError(t *testing.T) {
	task := tasks.DefineResultTask("bad-json", func(_ context.Context, _ resultPayload) (resultOutput, error) {
		t.Fatal("handler should not be called on unmarshal error")
		return resultOutput{}, nil
	})

	q := newMockQueue()
	task.Register(q)

	handler := q.handlers["bad-json"]
	err := handler(context.Background(), []byte(`not valid json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
	assert.Contains(t, err.Error(), "bad-json")
}

func TestResultCapture_NilContext(t *testing.T) {
	// CaptureResult should be a no-op when no capture is in context.
	assert.NotPanics(t, func() {
		tasks.CaptureResult(context.Background(), []byte(`{"ok":true}`))
	})
}

func TestResultCapture_NoCapture(t *testing.T) {
	capture := &tasks.ResultCapture{}
	assert.Empty(t, capture.Result())
}

func TestResultTask_RegisterPassesOptions(t *testing.T) {
	task := tasks.DefineResultTask("with-opts", func(_ context.Context, _ resultPayload) (resultOutput, error) {
		return resultOutput{}, nil
	}, tasks.WithMaxRetries(10))

	q := newMockQueue()
	task.Register(q)

	require.Contains(t, q.opts, "with-opts")
	cfg := tasks.JobConfig{MaxRetries: 3}
	for _, o := range q.opts["with-opts"] {
		o(&cfg)
	}
	assert.Equal(t, 10, cfg.MaxRetries)
}
