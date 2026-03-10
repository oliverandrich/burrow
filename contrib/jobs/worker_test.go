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

func testWorkerConfig() WorkerConfig {
	return WorkerConfig{
		NumWorkers:   2,
		PollInterval: 10 * time.Millisecond,
		BatchSize:    10,
		StaleTimeout: 10 * time.Minute,
	}
}

func TestWorker_ProcessJob(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	var processed atomic.Int32
	handlers := map[string]burrow.JobHandlerFunc{
		"test": func(_ context.Context, _ []byte) error {
			processed.Add(1)
			return nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	w := NewWorker(repo, handlers, testWorkerConfig())

	// Enqueue a job.
	_, err := repo.Enqueue(context.Background(), "test", `{}`, 3, time.Now())
	require.NoError(t, err)

	go w.Start(ctx)

	require.Eventually(t, func() bool {
		return processed.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	<-w.Done()
}

func TestWorker_RetryOnFailure(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	var attempts atomic.Int32
	handlers := map[string]burrow.JobHandlerFunc{
		"flaky": func(_ context.Context, _ []byte) error {
			if attempts.Add(1) <= 2 {
				return fmt.Errorf("temporary error")
			}
			return nil
		},
	}

	// Use fast poll + short config so retries happen quickly.
	cfg := testWorkerConfig()
	ctx, cancel := context.WithCancel(context.Background())
	w := NewWorker(repo, handlers, cfg)

	// Enqueue with maxRetries=3.
	_, err := repo.Enqueue(context.Background(), "flaky", `{}`, 3, time.Now())
	require.NoError(t, err)

	go w.Start(ctx)

	// After retry backoff, the job should eventually succeed on attempt 3.
	// Backoff: 2^1=2s, 2^2=4s — too slow for tests. We'll manually reset run_at.
	require.Eventually(t, func() bool {
		// Speed up retries by resetting run_at to now for failed jobs awaiting retry.
		_, _ = db.ExecContext(context.Background(),
			"UPDATE _jobs SET run_at = datetime('now') WHERE status = 'failed'")
		return attempts.Load() >= 3
	}, 5*time.Second, 20*time.Millisecond)

	cancel()
	<-w.Done()

	// Verify the job completed.
	var job Job
	err = db.NewSelect().Model(&job).Limit(1).Scan(context.Background())
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, job.Status)
}

func TestWorker_DeadAfterMaxRetries(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	handlers := map[string]burrow.JobHandlerFunc{
		"always_fail": func(_ context.Context, _ []byte) error {
			return fmt.Errorf("permanent error")
		},
	}

	cfg := testWorkerConfig()
	ctx, cancel := context.WithCancel(context.Background())
	w := NewWorker(repo, handlers, cfg)

	// Enqueue with maxRetries=1 (only 1 attempt allowed).
	_, err := repo.Enqueue(context.Background(), "always_fail", `{}`, 1, time.Now())
	require.NoError(t, err)

	go w.Start(ctx)

	require.Eventually(t, func() bool {
		var job Job
		if err := db.NewSelect().Model(&job).Limit(1).Scan(context.Background()); err != nil {
			return false
		}
		return job.Status == StatusDead
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	<-w.Done()
}

func TestWorker_UnknownType(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	handlers := map[string]burrow.JobHandlerFunc{} // No handlers registered.

	cfg := testWorkerConfig()
	ctx, cancel := context.WithCancel(context.Background())
	w := NewWorker(repo, handlers, cfg)

	_, err := repo.Enqueue(context.Background(), "nonexistent", `{}`, 3, time.Now())
	require.NoError(t, err)

	go w.Start(ctx)

	require.Eventually(t, func() bool {
		var job Job
		if err := db.NewSelect().Model(&job).Limit(1).Scan(context.Background()); err != nil {
			return false
		}
		return job.Status == StatusDead
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	<-w.Done()
}

func TestWorker_GracefulShutdown(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	started := make(chan struct{})
	handlers := map[string]burrow.JobHandlerFunc{
		"slow": func(_ context.Context, _ []byte) error {
			close(started)
			time.Sleep(100 * time.Millisecond)
			return nil
		},
	}

	cfg := testWorkerConfig()
	ctx, cancel := context.WithCancel(context.Background())
	w := NewWorker(repo, handlers, cfg)

	_, err := repo.Enqueue(context.Background(), "slow", `{}`, 3, time.Now())
	require.NoError(t, err)

	go w.Start(ctx)

	// Wait for the handler to start.
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}

	// Cancel while job is in-flight.
	cancel()

	// Worker should finish the in-flight job and then stop.
	select {
	case <-w.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not shut down")
	}

	// Verify the job completed.
	var job Job
	err = db.NewSelect().Model(&job).Limit(1).Scan(context.Background())
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, job.Status)
}

func TestDefaultWorkerConfig(t *testing.T) {
	cfg := DefaultWorkerConfig()
	assert.Equal(t, 2, cfg.NumWorkers)
	assert.Equal(t, time.Second, cfg.PollInterval)
	assert.Equal(t, 10, cfg.BatchSize)
	assert.Equal(t, 10*time.Minute, cfg.StaleTimeout)
}

func TestNewWorker(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	handlers := map[string]burrow.JobHandlerFunc{}
	cfg := DefaultWorkerConfig()

	w := NewWorker(repo, handlers, cfg)
	require.NotNil(t, w)
	assert.Equal(t, cfg.NumWorkers, w.config.NumWorkers)
	assert.Equal(t, cfg.PollInterval, w.config.PollInterval)
	assert.Equal(t, cfg.BatchSize, w.config.BatchSize)
}

func TestWorker_Maintenance(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	handlers := map[string]burrow.JobHandlerFunc{}
	cfg := testWorkerConfig()
	w := NewWorker(repo, handlers, cfg)

	// Create a stale running job (locked 30 min ago).
	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)
	claimed, err := repo.Claim(ctx, 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	_, err = db.NewUpdate().Model((*Job)(nil)).
		Set("locked_at = ?", time.Now().Add(-30*time.Minute)).
		Where("id = ?", job.ID).Exec(ctx)
	require.NoError(t, err)

	// Create a completed job older than 24h.
	job2, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)
	require.NoError(t, repo.Complete(ctx, job2.ID))
	_, err = db.NewUpdate().Model((*Job)(nil)).
		Set("completed_at = ?", time.Now().Add(-48*time.Hour)).
		Where("id = ?", job2.ID).Exec(ctx)
	require.NoError(t, err)

	// Run maintenance directly.
	w.maintenance(ctx)

	// Stale job should be rescued (back to pending).
	got, err := repo.GetByID(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, got.Status)

	// Completed job should be deleted.
	_, err = repo.GetByID(ctx, job2.ID)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestWorker_ScheduledJob(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	var processed atomic.Int32
	handlers := map[string]burrow.JobHandlerFunc{
		"scheduled": func(_ context.Context, _ []byte) error {
			processed.Add(1)
			return nil
		},
	}

	cfg := testWorkerConfig()
	ctx, cancel := context.WithCancel(context.Background())
	w := NewWorker(repo, handlers, cfg)

	// Schedule for 100ms in the future.
	_, err := repo.Enqueue(context.Background(), "scheduled", `{}`, 3, time.Now().Add(100*time.Millisecond))
	require.NoError(t, err)

	go w.Start(ctx)

	// Should not be processed immediately.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(0), processed.Load())

	// Should be processed after the scheduled time.
	require.Eventually(t, func() bool {
		return processed.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	<-w.Done()
}
