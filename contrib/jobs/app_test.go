package jobs

import (
	"context"
	"html/template"
	"io/fs"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApp_InterfaceAssertions(t *testing.T) {
	app := New()
	assert.Implements(t, (*burrow.App)(nil), app)
	assert.Implements(t, (*burrow.Queue)(nil), app)
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

	app.Handle("test_job", func(_ context.Context, _ []byte) error {
		return nil
	}, burrow.WithMaxRetries(5))

	jobID, err := app.Enqueue(context.Background(), "test_job", map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.NotEmpty(t, jobID)

	// Verify the job was stored correctly.
	id, _ := strconv.ParseInt(jobID, 10, 64)
	job, err := app.repo.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "test_job", job.Type)
	assert.JSONEq(t, `{"key":"value"}`, job.Payload)
	assert.Equal(t, 5, job.MaxRetries)
}

func TestApp_EnqueueAt(t *testing.T) {
	db := testDB(t)
	app := New()
	app.repo = NewRepository(db)

	app.Handle("delayed", func(_ context.Context, _ []byte) error {
		return nil
	})

	future := time.Now().Add(time.Hour)
	jobID, err := app.EnqueueAt(context.Background(), "delayed", "payload", future)
	require.NoError(t, err)

	id, _ := strconv.ParseInt(jobID, 10, 64)
	job, err := app.repo.GetByID(context.Background(), id)
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

	app.Handle("test", func(_ context.Context, _ []byte) error { return nil })

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
	app.Handle("lifecycle", func(_ context.Context, _ []byte) error {
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
	flags := app.Flags(nil)
	assert.Len(t, flags, 2)
}

func TestApp_Dequeue(t *testing.T) {
	db := testDB(t)
	app := New()
	app.repo = NewRepository(db)

	app.Handle("task", func(_ context.Context, _ []byte) error { return nil })

	jobID, err := app.Enqueue(context.Background(), "task", nil)
	require.NoError(t, err)

	err = app.Dequeue(context.Background(), jobID)
	require.NoError(t, err)
}

func TestApp_Dequeue_InvalidID(t *testing.T) {
	app := New()
	app.repo = NewRepository(nil) // repo won't be reached

	err := app.Dequeue(context.Background(), "not-a-number")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid job ID")
}

func TestApp_AdminRoutes_NilJobsAdmin(t *testing.T) {
	app := New()
	// jobsAdmin is nil when Register has not been called.
	r := chi.NewRouter()
	// Should not panic.
	app.AdminRoutes(r)
}

func TestApp_AdminRoutes_WithJobsAdmin(t *testing.T) {
	db := testDB(t)
	app := New()
	err := app.Register(&burrow.AppConfig{DB: db})
	require.NoError(t, err)

	r := chi.NewRouter()
	// Should not panic; registers routes on the router.
	app.AdminRoutes(r)
}

func TestApp_AdminNavItems(t *testing.T) {
	app := New()
	items := app.AdminNavItems()
	require.Len(t, items, 1)
	assert.Equal(t, "Jobs", items[0].Label)
	assert.Equal(t, "/admin/jobs", items[0].URL)
	assert.True(t, items[0].AdminOnly)
}

func TestApp_TemplateFS(t *testing.T) {
	app := New()
	fsys := app.TemplateFS()
	require.NotNil(t, fsys)

	entries, err := fs.ReadDir(fsys, ".")
	require.NoError(t, err)
	assert.NotEmpty(t, entries)

	// All entries should be .html files.
	for _, e := range entries {
		assert.Contains(t, e.Name(), ".html")
	}
}

func TestApp_FuncMap(t *testing.T) {
	app := New()
	fm := app.FuncMap()
	require.NotNil(t, fm)

	// Verify expected keys exist.
	expectedKeys := []string{"prettyJSON", "jobStatus", "string", "iconArrowCounterclockwise", "iconXCircle", "iconTrash"}
	for _, key := range expectedKeys {
		assert.Contains(t, fm, key)
	}

	// Test jobStatus function.
	jobStatusFn := fm["jobStatus"].(func(Job) string)
	assert.Equal(t, "pending", jobStatusFn(Job{Status: StatusPending}))
	assert.Equal(t, "dead", jobStatusFn(Job{Status: StatusDead}))

	// Test string function.
	stringFn := fm["string"].(func(any) string)
	assert.Equal(t, "42", stringFn(42))
	assert.Equal(t, "hello", stringFn("hello"))

	// Test icon functions return non-empty template.HTML.
	iconFn := fm["iconArrowCounterclockwise"].(func(...string) template.HTML)
	assert.NotEmpty(t, iconFn())

	iconXFn := fm["iconXCircle"].(func(...string) template.HTML)
	assert.NotEmpty(t, iconXFn())

	iconTrashFn := fm["iconTrash"].(func(...string) template.HTML)
	assert.NotEmpty(t, iconTrashFn())
}

func TestPrettyJSON(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		result := prettyJSON(`{"key":"value","num":42}`)
		assert.Contains(t, result, "  ")  // indented
		assert.Contains(t, result, "key") // content preserved
		assert.Contains(t, result, "value")
	})

	t.Run("invalid JSON returns as-is", func(t *testing.T) {
		input := "not json at all"
		result := prettyJSON(input)
		assert.Equal(t, input, result)
	})

	t.Run("empty object", func(t *testing.T) {
		result := prettyJSON(`{}`)
		assert.Equal(t, "{}", result)
	})
}

func TestApp_TranslationFS(t *testing.T) {
	app := New()
	fsys := app.TranslationFS()
	require.NotNil(t, fsys)
}

func TestApp_Shutdown_NilFields(t *testing.T) {
	app := New()
	// cancelFunc and worker are nil — should not panic.
	err := app.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestApp_Shutdown_NilWorker(t *testing.T) {
	cancelled := false
	app := New()
	app.cancelFunc = func() { cancelled = true }
	// worker is nil.
	err := app.Shutdown(context.Background())
	require.NoError(t, err)
	assert.True(t, cancelled)
}

func TestApp_Handle_DefaultRetries(t *testing.T) {
	app := New()
	app.Handle("test", func(_ context.Context, _ []byte) error { return nil })

	assert.Equal(t, 3, app.retries["test"])
	assert.NotNil(t, app.handlers["test"])
}
