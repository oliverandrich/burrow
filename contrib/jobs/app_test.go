package jobs

import (
	"context"
	"io/fs"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApp_InterfaceAssertions(t *testing.T) {
	app := New()
	assert.Implements(t, (*burrow.App)(nil), app)
	assert.Implements(t, (*burrow.Migratable)(nil), app)
	assert.Implements(t, (*burrow.Configurable)(nil), app)
	assert.Implements(t, (*burrow.HasShutdown)(nil), app)
}

func TestApp_Name(t *testing.T) {
	app := New()
	assert.Equal(t, "jobs", app.Name())
}

func TestApp_HandleAndEnqueue(t *testing.T) {
	db := testDB(t)
	app := New()
	app.repo = NewRepository(db)

	app.Handle("test_job", func(_ context.Context, _ *Job) error {
		return nil
	}, WithMaxRetries(5))

	job, err := app.Enqueue(context.Background(), "test_job", map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.Equal(t, "test_job", job.Type)
	assert.JSONEq(t, `{"key":"value"}`, job.Payload)
	assert.Equal(t, 5, job.MaxRetries)
}

func TestApp_EnqueueAt(t *testing.T) {
	db := testDB(t)
	app := New()
	app.repo = NewRepository(db)

	app.Handle("delayed", func(_ context.Context, _ *Job) error {
		return nil
	})

	future := time.Now().Add(time.Hour)
	job, err := app.EnqueueAt(context.Background(), "delayed", "payload", future)
	require.NoError(t, err)
	assert.WithinDuration(t, future, job.RunAt, time.Second)
}

func TestApp_Enqueue_UnknownType(t *testing.T) {
	db := testDB(t)
	app := New()
	app.repo = NewRepository(db)

	_, err := app.Enqueue(context.Background(), "nonexistent", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown type")
}

func TestApp_Enqueue_InvalidPayload(t *testing.T) {
	db := testDB(t)
	app := New()
	app.repo = NewRepository(db)

	app.Handle("test", func(_ context.Context, _ *Job) error { return nil })

	// Channels cannot be marshaled to JSON.
	_, err := app.Enqueue(context.Background(), "test", make(chan int))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal payload")
}

func TestApp_FullLifecycle(t *testing.T) {
	db := testDB(t)
	app := New()
	app.repo = NewRepository(db)

	var processed atomic.Int32
	app.Handle("lifecycle", func(_ context.Context, _ *Job) error {
		processed.Add(1)
		return nil
	})

	// Simulate Configure — start the worker directly.
	cfg := testWorkerConfig()
	ctx, cancel := context.WithCancel(context.Background())
	app.cancelFunc = cancel
	app.worker = NewWorker(app.repo, app.handlers, cfg)
	go app.worker.Start(ctx)

	// Enqueue a job.
	_, err := app.Enqueue(context.Background(), "lifecycle", nil)
	require.NoError(t, err)

	// Wait for processing.
	require.Eventually(t, func() bool {
		return processed.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)

	// Shutdown.
	err = app.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestApp_MigrationFS(t *testing.T) {
	app := New()
	fsys := app.MigrationFS()
	require.NotNil(t, fsys)

	// Should contain our migration file.
	entries, err := fs.ReadDir(fsys, ".")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "001_initial_schema.up.sql", entries[0].Name())
}

func TestApp_Flags(t *testing.T) {
	app := New()
	flags := app.Flags()
	assert.Len(t, flags, 2)
}
