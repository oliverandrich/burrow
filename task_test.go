package burrow_test

import (
	"context"
	"testing"
	"time"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
}

// mockQueue records Handle and Enqueue calls for testing TaskDefinition.
type mockQueue struct {
	handlers map[string]burrow.JobHandlerFunc
	opts     map[string][]burrow.JobOption
	lastType string
	lastData any
}

func newMockQueue() *mockQueue {
	return &mockQueue{
		handlers: make(map[string]burrow.JobHandlerFunc),
		opts:     make(map[string][]burrow.JobOption),
	}
}

func (q *mockQueue) Handle(typeName string, fn burrow.JobHandlerFunc, opts ...burrow.JobOption) {
	q.handlers[typeName] = fn
	q.opts[typeName] = opts
}

func (q *mockQueue) Enqueue(ctx context.Context, typeName string, payload any) (string, error) {
	q.lastType = typeName
	q.lastData = payload
	return "job-123", nil
}

func (q *mockQueue) EnqueueAt(ctx context.Context, typeName string, payload any, runAt time.Time) (string, error) {
	q.lastType = typeName
	q.lastData = payload
	return "job-456", nil
}

func (q *mockQueue) Dequeue(ctx context.Context, id string) error {
	return nil
}

func TestTaskDefinition_Name(t *testing.T) {
	task := burrow.DefineTask("send-email", func(_ context.Context, _ testPayload) error {
		return nil
	})
	assert.Equal(t, "send-email", task.Name())
}

func TestTaskDefinition_RegisterAndEnqueue(t *testing.T) {
	var received testPayload
	task := burrow.DefineTask("send-email", func(_ context.Context, p testPayload) error {
		received = p
		return nil
	})

	q := newMockQueue()
	task.Register(q)

	// Verify handler was registered.
	require.Contains(t, q.handlers, "send-email")

	// Enqueue a typed payload.
	id, err := task.Enqueue(context.Background(), testPayload{To: "x@y.com", Subject: "hello"})
	require.NoError(t, err)
	assert.Equal(t, "job-123", id)
	assert.Equal(t, "send-email", q.lastType)

	// Simulate what the real queue does: marshal payload, call handler with bytes.
	// The handler registered via Register should unmarshal and call our typed handler.
	handler := q.handlers["send-email"]
	err = handler(context.Background(), []byte(`{"to":"x@y.com","subject":"hello"}`))
	require.NoError(t, err)
	assert.Equal(t, "x@y.com", received.To)
	assert.Equal(t, "hello", received.Subject)
}

func TestTaskDefinition_EnqueueAt(t *testing.T) {
	task := burrow.DefineTask("delayed-task", func(_ context.Context, _ testPayload) error {
		return nil
	})

	q := newMockQueue()
	task.Register(q)

	runAt := time.Now().Add(time.Hour)
	id, err := task.EnqueueAt(context.Background(), testPayload{To: "a@b.com"}, runAt)
	require.NoError(t, err)
	assert.Equal(t, "job-456", id)
	assert.Equal(t, "delayed-task", q.lastType)
}

func TestTaskDefinition_EnqueueBeforeRegister(t *testing.T) {
	task := burrow.DefineTask("unregistered", func(_ context.Context, _ testPayload) error {
		return nil
	})

	assert.PanicsWithValue(t,
		`burrow: TaskDefinition "unregistered" used before Register was called`,
		func() {
			_, _ = task.Enqueue(context.Background(), testPayload{})
		},
	)
}

func TestTaskDefinition_EnqueueAtBeforeRegister(t *testing.T) {
	task := burrow.DefineTask("unregistered", func(_ context.Context, _ testPayload) error {
		return nil
	})

	assert.PanicsWithValue(t,
		`burrow: TaskDefinition "unregistered" used before Register was called`,
		func() {
			_, _ = task.EnqueueAt(context.Background(), testPayload{}, time.Now())
		},
	)
}

func TestTaskDefinition_HandlerUnmarshalError(t *testing.T) {
	task := burrow.DefineTask("bad-json", func(_ context.Context, _ testPayload) error {
		t.Fatal("handler should not be called on unmarshal error")
		return nil
	})

	q := newMockQueue()
	task.Register(q)

	handler := q.handlers["bad-json"]
	err := handler(context.Background(), []byte(`not valid json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
	assert.Contains(t, err.Error(), "bad-json")
}

func TestTaskDefinition_RegisterPassesOptions(t *testing.T) {
	task := burrow.DefineTask("with-opts", func(_ context.Context, _ testPayload) error {
		return nil
	}, burrow.WithMaxRetries(7))

	q := newMockQueue()
	task.Register(q)

	require.Contains(t, q.opts, "with-opts")
	// Verify options were passed through by applying them.
	cfg := burrow.JobConfig{MaxRetries: 3}
	for _, o := range q.opts["with-opts"] {
		o(&cfg)
	}
	assert.Equal(t, 7, cfg.MaxRetries)
}
