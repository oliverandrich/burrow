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
	require.NoError(t, repo.Fail(ctx, job.ID, "boom", 3, 3)) // dead

	handler := retryHandler(repo)

	r := chi.NewRouter()
	r.Post("/admin/jobs/{id}/retry", burrow.Handle(handler))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/admin/jobs/%d/retry", job.ID), nil)
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

	_, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now()) // pending
	require.NoError(t, err)

	handler := retryHandler(repo)

	r := chi.NewRouter()
	r.Post("/admin/jobs/{id}/retry", burrow.Handle(handler))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/jobs/1/retry", nil)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/admin/jobs/%d/cancel", job.ID), nil)
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
	require.NoError(t, repo.Complete(ctx, job.ID)) // completed

	handler := cancelHandler(repo)

	r := chi.NewRouter()
	r.Post("/admin/jobs/{id}/cancel", burrow.Handle(handler))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/jobs/1/cancel", nil)
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
