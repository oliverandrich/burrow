package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time assertion: the jobs app reports worker liveness via readiness.
var _ burrow.ReadinessChecker = (*App)(nil)

func TestPollStale(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	const threshold = 30 * time.Second

	tests := []struct {
		name    string
		lastAgo time.Duration
		want    bool
	}{
		{"fresh", time.Second, false},
		{"just under threshold", 29 * time.Second, false},
		{"exactly at threshold", 30 * time.Second, false},
		{"just over threshold", 31 * time.Second, true},
		{"long stalled", 5 * time.Minute, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, pollStale(now.Add(-tc.lastAgo), now, threshold))
		})
	}
}

func TestReadinessThreshold(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		want     time.Duration
	}{
		{"sub-second floored to 30s", time.Second, 30 * time.Second},
		{"default 1s floored to 30s", time.Second, 30 * time.Second},
		{"3s floored to 30s", 3 * time.Second, 30 * time.Second},
		{"10s scales to 100s", 10 * time.Second, 100 * time.Second},
		{"zero floored to 30s", 0, 30 * time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, readinessThreshold(tc.interval))
		})
	}
}

func TestWorker_Stats(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	release := make(chan struct{})
	started := make(chan struct{}, 1)
	handlers := map[string]burrow.JobHandlerFunc{
		"block": func(_ context.Context, _ []byte) error {
			started <- struct{}{}
			<-release
			return nil
		},
	}

	cfg := testWorkerConfig()
	cfg.NumWorkers = 3
	w := NewWorker(repo, handlers, cfg, nil)

	// Before Start: not running, nothing in flight.
	pre := w.Stats()
	assert.False(t, pre.Running)
	assert.Equal(t, 3, pre.NumWorkers)
	assert.Equal(t, 0, pre.InFlight)
	assert.NotEmpty(t, pre.WorkerID)

	_, err := repo.Enqueue(context.Background(), "block", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	// While the handler blocks: running, one job in flight, poll timestamp advancing.
	<-started
	require.Eventually(t, func() bool {
		s := w.Stats()
		return s.Running && s.InFlight == 1 && time.Since(s.LastPollAt) < time.Second
	}, 2*time.Second, 10*time.Millisecond)

	close(release)
	require.Eventually(t, func() bool { return w.Stats().InFlight == 0 }, 2*time.Second, 10*time.Millisecond)

	cancel()
	<-w.Done()
	assert.False(t, w.Stats().Running, "worker reports stopped after Done")
}

func TestApp_ReadinessCheck_NilWorker(t *testing.T) {
	app := New()
	err := app.ReadinessCheck(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}

func TestApp_ReadinessCheck_FreshPoll(t *testing.T) {
	db := testDB(t)
	app := New()
	app.repo = NewRepository(db)
	app.workerCfg = DefaultWorkerConfig()
	app.worker = NewWorker(app.repo, app.handlers, app.workerCfg, nil)
	app.worker.lastPollAt.Store(time.Now().UnixNano())

	require.NoError(t, app.ReadinessCheck(t.Context()))
}

func TestApp_ReadinessCheck_StalePoll(t *testing.T) {
	db := testDB(t)
	app := New()
	app.repo = NewRepository(db)
	app.workerCfg = DefaultWorkerConfig()
	app.worker = NewWorker(app.repo, app.handlers, app.workerCfg, nil)
	app.worker.lastPollAt.Store(time.Now().Add(-5 * time.Minute).UnixNano())

	err := app.ReadinessCheck(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stalled")
}
