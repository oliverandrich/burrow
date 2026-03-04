package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codeberg.org/oliverandrich/burrow"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRenderer captures the arguments passed to admin render methods.
type stubRenderer struct { //nolint:govet // fieldalignment: test struct, readability over optimization
	listJobs   []Job
	listPage   burrow.PageResult
	listStatus string
	detailJob  *Job
}

func (s *stubRenderer) AdminJobsListPage(_ http.ResponseWriter, _ *http.Request, jobs []Job, page burrow.PageResult, activeStatus string) error {
	s.listJobs = jobs
	s.listPage = page
	s.listStatus = activeStatus
	return nil
}

func (s *stubRenderer) AdminJobDetailPage(_ http.ResponseWriter, _ *http.Request, job *Job) error {
	s.detailJob = job
	return nil
}

// setupAdminTest creates an in-memory DB, repo, and admin handlers.
func setupAdminTest(t *testing.T) (*adminHandlers, *Repository, *stubRenderer) {
	t.Helper()
	db := testDB(t)
	repo := NewRepository(db)
	renderer := &stubRenderer{}
	handlers := newAdminHandlers(repo, renderer)
	return handlers, repo, renderer
}

// chiRequest builds an http.Request with chi URL params set.
func chiRequest(method, path string, params map[string]string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestAdminHandlers_ListPage(t *testing.T) {
	h, repo, renderer := setupAdminTest(t)
	ctx := context.Background()

	for range 3 {
		_, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
		require.NoError(t, err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin/jobs?page=1&limit=10", nil)

	err := h.ListPage(w, r)
	require.NoError(t, err)
	assert.Len(t, renderer.listJobs, 3)
	assert.Equal(t, 3, renderer.listPage.TotalCount)
}

func TestAdminHandlers_ListPage_StatusFilter(t *testing.T) {
	h, repo, renderer := setupAdminTest(t)
	ctx := context.Background()

	j, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)
	require.NoError(t, repo.Complete(ctx, j.ID))
	_, err = repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/admin/jobs?status=pending", nil)

	err = h.ListPage(w, r)
	require.NoError(t, err)
	assert.Len(t, renderer.listJobs, 1)
	assert.Equal(t, "pending", renderer.listStatus)
}

func TestAdminHandlers_DetailPage(t *testing.T) {
	h, repo, renderer := setupAdminTest(t)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "email", `{"to":"a@b.com"}`, 3, time.Now())
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := chiRequest(http.MethodGet, "/admin/jobs/1", map[string]string{"id": "1"})

	err = h.DetailPage(w, r)
	require.NoError(t, err)
	require.NotNil(t, renderer.detailJob)
	assert.Equal(t, job.ID, renderer.detailJob.ID)
}

func TestAdminHandlers_DetailPage_NotFound(t *testing.T) {
	h, _, _ := setupAdminTest(t)

	w := httptest.NewRecorder()
	r := chiRequest(http.MethodGet, "/admin/jobs/9999", map[string]string{"id": "9999"})

	err := h.DetailPage(w, r)
	require.Error(t, err)
	var httpErr *burrow.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusNotFound, httpErr.Code)
}

func TestAdminHandlers_DeleteJob(t *testing.T) {
	h, repo, _ := setupAdminTest(t)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := chiRequest(http.MethodDelete, "/admin/jobs/1", map[string]string{"id": "1"})

	err = h.DeleteJob(w, r)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "/admin/jobs", w.Header().Get("HX-Redirect"))

	// Verify deleted.
	_, err = repo.GetByID(ctx, job.ID)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestAdminHandlers_RetryJob(t *testing.T) {
	h, repo, _ := setupAdminTest(t)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)
	require.NoError(t, repo.Fail(ctx, job.ID, "boom", 3, 3)) // dead

	w := httptest.NewRecorder()
	r := chiRequest(http.MethodPost, "/admin/jobs/1/retry", map[string]string{"id": "1"})

	err = h.RetryJob(w, r)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("HX-Redirect"), "/admin/jobs/")

	got, err := repo.GetByID(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, got.Status)
}

func TestAdminHandlers_RetryJob_InvalidStatus(t *testing.T) {
	h, repo, _ := setupAdminTest(t)
	ctx := context.Background()

	_, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now()) // pending
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := chiRequest(http.MethodPost, "/admin/jobs/1/retry", map[string]string{"id": "1"})

	err = h.RetryJob(w, r)
	require.Error(t, err)
	var httpErr *burrow.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
}

func TestAdminHandlers_CancelJob(t *testing.T) {
	h, repo, _ := setupAdminTest(t)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := chiRequest(http.MethodPost, "/admin/jobs/1/cancel", map[string]string{"id": "1"})

	err = h.CancelJob(w, r)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	got, err := repo.GetByID(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusDead, got.Status)
}

func TestAdminHandlers_CancelJob_InvalidStatus(t *testing.T) {
	h, repo, _ := setupAdminTest(t)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)
	require.NoError(t, repo.Complete(ctx, job.ID)) // completed

	w := httptest.NewRecorder()
	r := chiRequest(http.MethodPost, "/admin/jobs/1/cancel", map[string]string{"id": "1"})

	err = h.CancelJob(w, r)
	require.Error(t, err)
	var httpErr *burrow.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
}
