package jobs

import (
	"bytes"
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renderAdminList parses the real jobs templates (admin_list provides the
// worker-status card, admin_detail provides the status_badge it calls) and
// executes the card with the given view. It pins that every field and method
// the card references actually resolves — the auth gate hides this path from
// the live-server smoke test.
func renderAdminList(t *testing.T, view workerStatusView) string {
	t.Helper()
	app := &App{}
	funcs := app.FuncMap() // real prettyJSON/jobStatus/string used across the jobs templates
	funcs["t"] = func(s string) string { return s }
	funcs["dict"] = func(...any) map[string]any { return map[string]any{} }
	tmpl, err := template.New("jobs").Funcs(funcs).ParseFS(app.TemplateFS(), "*.html")
	require.NoError(t, err)
	// admin/pagination lives in the admin contrib; stub it so html/template's
	// escaping pass resolves the (unreached, empty-list) reference.
	_, err = tmpl.Parse(`{{ define "admin/pagination" }}{{ end }}`)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, tmpl.ExecuteTemplate(&buf, "jobs/admin_list", map[string]any{
		"Jobs":     nil, // empty list skips the pagination template (defined in admin)
		"Status":   "",
		"RawQuery": "",
		"Worker":   view,
	}))
	return buf.String()
}

func TestAdminListCard_RendersRunningWorker(t *testing.T) {
	view := workerStatusView{
		Started: true, Running: true, Workers: 4, InFlight: 2,
		LastPollAt: time.Now(),
		Counts:     []statusCount{{StatusPending, 7}, {StatusRunning, 2}, {StatusFailed, 1}, {StatusDead, 0}, {StatusCompleted, 99}},
	}
	out := renderAdminList(t, view)
	assert.Contains(t, out, "Worker Status")
	assert.Contains(t, out, "Running")
	assert.Contains(t, out, "4")  // worker count
	assert.Contains(t, out, "99") // completed count
	assert.NotContains(t, out, "Not Started")
}

func TestAdminListCard_RendersNotStarted(t *testing.T) {
	view := workerStatusView{
		Counts: []statusCount{{StatusPending, 0}, {StatusRunning, 0}, {StatusFailed, 0}, {StatusDead, 0}, {StatusCompleted, 0}},
	}
	out := renderAdminList(t, view)
	assert.Contains(t, out, "Not Started")
	// The worker-meta row (Workers/In Flight/Last Poll) is hidden when not started.
	assert.NotContains(t, out, "In Flight")
}

func TestAdminListCard_RendersStaleBadge(t *testing.T) {
	view := workerStatusView{
		Started: true, Running: true, Stale: true, Workers: 1,
		LastPollAt: time.Now().Add(-5 * time.Minute),
		Counts:     []statusCount{{StatusPending, 0}, {StatusRunning, 0}, {StatusFailed, 0}, {StatusDead, 0}, {StatusCompleted, 0}},
	}
	out := renderAdminList(t, view)
	// The "Stalled" badge text only appears when the stale branch renders.
	assert.Equal(t, 1, strings.Count(out, ">Stalled<"), "stalled badge rendered exactly once")
}

func TestCountByStatus(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	for range 3 {
		_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
		require.NoError(t, err)
	}

	counts, err := repo.CountByStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, counts[StatusPending])
	assert.Equal(t, 0, counts[StatusRunning])
	assert.Equal(t, 0, counts[StatusDead])

	// Claiming a job flips it from pending to running.
	claimed, err := repo.Claim(ctx, "w1", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	counts, err = repo.CountByStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, counts[StatusPending])
	assert.Equal(t, 1, counts[StatusRunning])
}

func countFor(view workerStatusView, s JobStatus) int {
	for _, c := range view.Counts {
		if c.Status == s {
			return c.Count
		}
	}
	return -1
}

func TestWorkerStatus_NilWorker(t *testing.T) {
	app := newTestApp(t)
	_, err := app.repo.Enqueue(context.Background(), "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	view, err := app.workerStatus(t.Context())
	require.NoError(t, err)
	assert.False(t, view.Started, "no worker => not started")
	assert.Equal(t, 0, view.Workers)
	require.Len(t, view.Counts, 5, "all five statuses always represented")
	assert.Equal(t, 1, countFor(view, StatusPending))
	assert.Equal(t, 0, countFor(view, StatusRunning))
}

func TestWorkerStatus_WithWorker(t *testing.T) {
	app := newTestApp(t)
	app.workerCfg = DefaultWorkerConfig()
	app.worker = NewWorker(app.repo, app.handlers, app.workerCfg, nil)
	app.worker.running.Store(true)
	app.worker.lastPollAt.Store(time.Now().UnixNano())

	view, err := app.workerStatus(t.Context())
	require.NoError(t, err)
	assert.True(t, view.Started)
	assert.True(t, view.Running)
	assert.Equal(t, app.workerCfg.NumWorkers, view.Workers)
	assert.False(t, view.Stale, "fresh poll is not stale")
}

func TestWorkerStatus_StaleWorker(t *testing.T) {
	app := newTestApp(t)
	app.workerCfg = DefaultWorkerConfig()
	app.worker = NewWorker(app.repo, app.handlers, app.workerCfg, nil)
	app.worker.running.Store(true)
	app.worker.lastPollAt.Store(time.Now().Add(-5 * time.Minute).UnixNano())

	view, err := app.workerStatus(t.Context())
	require.NoError(t, err)
	assert.True(t, view.Stale, "5-minute-old poll is stale")
}

func TestAdminListJobs_IncludesWorkerStatus(t *testing.T) {
	app := newTestApp(t)
	_, err := app.repo.Enqueue(context.Background(), "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	var captured map[string]any
	record := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithTemplateExecutor(r.Context(),
				func(_ context.Context, _ string, data map[string]any) (template.HTML, error) {
					captured = data
					return "", nil
				})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	r := chi.NewRouter()
	r.Use(record)
	r.Get("/admin/jobs", burrow.Handle(app.adminListJobs))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/jobs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, captured, "Worker")
	view, ok := captured["Worker"].(workerStatusView)
	require.True(t, ok, "Worker render value is a workerStatusView")
	assert.False(t, view.Started)
	assert.Len(t, view.Counts, 5)
}
