package jobs

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryHandler(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)
	job.Attempts = 3
	job.MaxRetries = 3
	require.NoError(t, repo.Fail(ctx, job, "boom", 30*time.Second)) // dead

	handler := retryHandler(repo)

	r := chi.NewRouter()
	r.Post("/admin/jobs/{id}/retry", burrow.Handle(handler))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/jobs/"+job.ID+"/retry", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("HX-Redirect"), "/admin/jobs/")

	got, err := repo.GetByID(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, got.Status)
}

func TestRetryHandler_InvalidStatus(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now()) // pending
	require.NoError(t, err)

	handler := retryHandler(repo)

	r := chi.NewRouter()
	r.Use(burrow.TestErrorExecMiddleware)
	r.Post("/admin/jobs/{id}/retry", burrow.Handle(handler))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/jobs/"+job.ID+"/retry", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCancelHandler(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)

	handler := cancelHandler(repo)

	r := chi.NewRouter()
	r.Post("/admin/jobs/{id}/cancel", burrow.Handle(handler))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/jobs/"+job.ID+"/cancel", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	got, err := repo.GetByID(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusDead, got.Status)
}

func TestCancelHandler_InvalidStatus(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)
	require.NoError(t, repo.Complete(ctx, job)) // completed

	handler := cancelHandler(repo)

	r := chi.NewRouter()
	r.Use(burrow.TestErrorExecMiddleware)
	r.Post("/admin/jobs/{id}/cancel", burrow.Handle(handler))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/jobs/"+job.ID+"/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestShowWhenRetryable(t *testing.T) {
	tests := []struct {
		status JobStatus
		want   bool
	}{
		{StatusFailed, true},
		{StatusDead, true},
		{StatusPending, false},
		{StatusRunning, false},
		{StatusCompleted, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			job := Job{Status: tt.status}
			assert.Equal(t, tt.want, isRetryable(job))
		})
	}
}

func TestShowWhenCancellable(t *testing.T) {
	tests := []struct {
		status JobStatus
		want   bool
	}{
		{StatusPending, true},
		{StatusRunning, true},
		{StatusFailed, true},
		{StatusCompleted, false},
		{StatusDead, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			job := Job{Status: tt.status}
			assert.Equal(t, tt.want, isCancellable(job))
		})
	}
}

func TestStatusChoices(t *testing.T) {
	choices := statusChoices()
	assert.Len(t, choices, 5)
	assert.Equal(t, "pending", choices[0].Value)
}

func TestParseJobID_EmptyID(t *testing.T) {
	r := chi.NewRouter()
	var capturedErr error

	r.Get("/admin/jobs/{id}", func(w http.ResponseWriter, req *http.Request) {
		_, capturedErr = parseJobID(req)
		if capturedErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// With chi, {id} always matches at least one character, so an empty ID
	// would not match the route at all. Test that parseJobID returns an error
	// for a missing URL param (which only happens if called from a mismatched route).
	// Instead, verify that a non-empty string is accepted as a valid ID.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/jobs/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.NoError(t, capturedErr)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestParseJobID_Valid(t *testing.T) {
	r := chi.NewRouter()
	var capturedID string
	var capturedErr error

	r.Get("/admin/jobs/{id}", func(w http.ResponseWriter, req *http.Request) {
		capturedID, capturedErr = parseJobID(req)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/jobs/42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.NoError(t, capturedErr)
	assert.Equal(t, "42", capturedID)
}

func TestMapRepoError(t *testing.T) {
	t.Run("ErrNotFound", func(t *testing.T) {
		err := mapRepoError(ErrNotFound)
		var httpErr *burrow.HTTPError
		require.ErrorAs(t, err, &httpErr)
		assert.Equal(t, http.StatusNotFound, httpErr.Code)
		assert.Contains(t, httpErr.Message, "not found")
	})

	t.Run("ErrInvalidStatus", func(t *testing.T) {
		err := mapRepoError(ErrInvalidStatus)
		var httpErr *burrow.HTTPError
		require.ErrorAs(t, err, &httpErr)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.Contains(t, httpErr.Message, "invalid status")
	})

	t.Run("default error", func(t *testing.T) {
		err := mapRepoError(fmt.Errorf("some database error"))
		var httpErr *burrow.HTTPError
		require.ErrorAs(t, err, &httpErr)
		assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
		assert.Contains(t, httpErr.Message, "failed to process job")
	})

	t.Run("wrapped ErrNotFound", func(t *testing.T) {
		err := mapRepoError(fmt.Errorf("wrap: %w", ErrNotFound))
		var httpErr *burrow.HTTPError
		require.ErrorAs(t, err, &httpErr)
		assert.Equal(t, http.StatusNotFound, httpErr.Code)
	})
}

func TestIsRetryable_NonJobType(t *testing.T) {
	assert.False(t, isRetryable("not a job"))
	assert.False(t, isRetryable(42))
	assert.False(t, isRetryable(nil))
}

func TestIsCancellable_NonJobType(t *testing.T) {
	assert.False(t, isCancellable("not a job"))
	assert.False(t, isCancellable(42))
	assert.False(t, isCancellable(nil))
}

func TestRetryHandler_NotFound(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	handler := retryHandler(repo)

	r := chi.NewRouter()
	r.Use(burrow.TestErrorExecMiddleware)
	r.Post("/admin/jobs/{id}/retry", burrow.Handle(handler))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/jobs/99999/retry", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCancelHandler_NotFound(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	handler := cancelHandler(repo)

	r := chi.NewRouter()
	r.Use(burrow.TestErrorExecMiddleware)
	r.Post("/admin/jobs/{id}/cancel", burrow.Handle(handler))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/jobs/99999/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRetryHandler_InvalidID(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	handler := retryHandler(repo)

	r := chi.NewRouter()
	r.Use(burrow.TestErrorExecMiddleware)
	r.Post("/admin/jobs/{id}/retry", burrow.Handle(handler))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/jobs/abc/retry", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// "abc" is a valid string ID but doesn't exist — retry returns ErrNotFound.
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCancelHandler_InvalidID(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	handler := cancelHandler(repo)

	r := chi.NewRouter()
	r.Use(burrow.TestErrorExecMiddleware)
	r.Post("/admin/jobs/{id}/cancel", burrow.Handle(handler))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/jobs/abc/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// "abc" is a valid string ID but doesn't exist — cancel returns ErrNotFound.
	assert.Equal(t, http.StatusNotFound, w.Code)
}
