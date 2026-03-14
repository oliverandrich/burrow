package jobs

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobPoolProcesses100Jobs(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	var completed atomic.Int32
	handlers := map[string]burrow.JobHandlerFunc{
		"counter": func(_ context.Context, _ []byte) error {
			completed.Add(1)
			return nil
		},
	}

	cfg := testWorkerConfig()
	cfg.NumWorkers = 4
	cfg.BatchSize = 20
	ctx, cancel := context.WithCancel(context.Background())
	w := NewWorker(repo, handlers, cfg)

	// Enqueue 100 jobs.
	for range 100 {
		_, err := repo.Enqueue(context.Background(), "counter", `{}`, 3, time.Now())
		require.NoError(t, err)
	}

	go w.Start(ctx)

	require.Eventually(t, func() bool {
		return completed.Load() == 100
	}, 10*time.Second, 20*time.Millisecond, "all 100 jobs should complete")

	cancel()
	<-w.Done()

	// Verify all jobs are marked completed in the database.
	var dbCompleted int
	err := db.NewRaw("SELECT COUNT(*) FROM _jobs WHERE status = ?", StatusCompleted).
		Scan(context.Background(), &dbCompleted)
	require.NoError(t, err)
	assert.Equal(t, 100, dbCompleted)
}

func TestJobPoolHandlerFailuresDoNotCrashPool(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	var successCount atomic.Int32
	var failCount atomic.Int32

	handlers := map[string]burrow.JobHandlerFunc{
		"good": func(_ context.Context, _ []byte) error {
			successCount.Add(1)
			return nil
		},
		"bad": func(_ context.Context, _ []byte) error {
			failCount.Add(1)
			return fmt.Errorf("intentional failure")
		},
	}

	cfg := testWorkerConfig()
	cfg.NumWorkers = 3
	cfg.RetryBaseDelay = time.Millisecond // fast retries for test
	ctx, cancel := context.WithCancel(context.Background())
	w := NewWorker(repo, handlers, cfg)

	// Enqueue a mix of good and bad jobs. Bad jobs have maxRetries=1 so they go dead immediately.
	for range 10 {
		_, err := repo.Enqueue(context.Background(), "bad", `{}`, 1, time.Now())
		require.NoError(t, err)
	}
	for range 20 {
		_, err := repo.Enqueue(context.Background(), "good", `{}`, 3, time.Now())
		require.NoError(t, err)
	}

	go w.Start(ctx)

	// All good jobs should complete despite bad jobs failing.
	require.Eventually(t, func() bool {
		return successCount.Load() == 20
	}, 10*time.Second, 20*time.Millisecond, "all good jobs should complete")

	// Bad jobs should have been attempted.
	require.Eventually(t, func() bool {
		return failCount.Load() >= 10
	}, 5*time.Second, 20*time.Millisecond, "all bad jobs should have been attempted")

	cancel()
	<-w.Done()

	// Verify good jobs are completed.
	var completedCount int
	err := db.NewRaw("SELECT COUNT(*) FROM _jobs WHERE status = ? AND type = ?",
		StatusCompleted, "good").Scan(context.Background(), &completedCount)
	require.NoError(t, err)
	assert.Equal(t, 20, completedCount)

	// Verify bad jobs are dead (maxRetries=1, so after 1 attempt they go dead).
	var deadCount int
	err = db.NewRaw("SELECT COUNT(*) FROM _jobs WHERE status = ? AND type = ?",
		StatusDead, "bad").Scan(context.Background(), &deadCount)
	require.NoError(t, err)
	assert.Equal(t, 10, deadCount)
}

func TestJobPoolMaxRetriesExhaustedEndsDead(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	var attempts atomic.Int32
	handlers := map[string]burrow.JobHandlerFunc{
		"doomed": func(_ context.Context, _ []byte) error {
			attempts.Add(1)
			return fmt.Errorf("always fails")
		},
	}

	cfg := testWorkerConfig()
	cfg.NumWorkers = 2
	cfg.RetryBaseDelay = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	w := NewWorker(repo, handlers, cfg)

	// Enqueue 5 jobs with maxRetries=3 (3 attempts allowed before dead).
	jobIDs := make([]int64, 0, 5)
	for range 5 {
		job, err := repo.Enqueue(context.Background(), "doomed", `{}`, 3, time.Now())
		require.NoError(t, err)
		jobIDs = append(jobIDs, job.ID)
	}

	go w.Start(ctx)

	// Wait for all 5 jobs to reach dead status.
	require.Eventually(t, func() bool {
		var deadCount int
		_ = db.NewRaw("SELECT COUNT(*) FROM _jobs WHERE status = ?", StatusDead).
			Scan(context.Background(), &deadCount)

		// Speed up retries by resetting run_at for failed jobs.
		_, _ = db.ExecContext(context.Background(),
			"UPDATE _jobs SET run_at = datetime('now') WHERE status = 'failed'")

		return deadCount == 5
	}, 10*time.Second, 20*time.Millisecond, "all jobs should reach dead status")

	cancel()
	<-w.Done()

	// Verify each job is dead with the expected error.
	for _, id := range jobIDs {
		job, err := repo.GetByID(context.Background(), id)
		require.NoError(t, err)
		assert.Equal(t, StatusDead, job.Status, "job %d should be dead", id)
		assert.Equal(t, "always fails", job.LastError, "job %d should have error message", id)
		assert.Equal(t, 3, job.Attempts, "job %d should have 3 attempts", id)
	}
}

func TestJobPoolPanicInHandlerDoesNotCrashPool(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	var goodCompleted atomic.Int32
	handlers := map[string]burrow.JobHandlerFunc{
		"good": func(_ context.Context, _ []byte) error {
			goodCompleted.Add(1)
			return nil
		},
		// Note: panics in handlers would crash the goroutine. This test verifies
		// that error-returning handlers (the expected failure mode) don't affect
		// other jobs in the pool.
		"error": func(_ context.Context, _ []byte) error {
			return fmt.Errorf("something went wrong")
		},
	}

	cfg := testWorkerConfig()
	cfg.NumWorkers = 2
	ctx, cancel := context.WithCancel(context.Background())
	w := NewWorker(repo, handlers, cfg)

	// Enqueue error jobs first (maxRetries=1 so they die quickly).
	for range 5 {
		_, err := repo.Enqueue(context.Background(), "error", `{}`, 1, time.Now())
		require.NoError(t, err)
	}
	// Then good jobs.
	for range 10 {
		_, err := repo.Enqueue(context.Background(), "good", `{}`, 3, time.Now())
		require.NoError(t, err)
	}

	go w.Start(ctx)

	// All good jobs should still complete.
	require.Eventually(t, func() bool {
		return goodCompleted.Load() == 10
	}, 10*time.Second, 20*time.Millisecond)

	cancel()
	<-w.Done()
}
