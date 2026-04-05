package jobs

import (
	"context"
	"fmt"
	"html/template"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/den"
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
	w := NewWorker(repo, handlers, testWorkerConfig(), nil)

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
	w := NewWorker(repo, handlers, cfg, nil)

	// Enqueue with maxRetries=3.
	_, err := repo.Enqueue(context.Background(), "flaky", `{}`, 3, time.Now())
	require.NoError(t, err)

	go w.Start(ctx)

	// After retry backoff, the job should eventually succeed on attempt 3.
	// Backoff: 2^1=2s, 2^2=4s — too slow for tests. We'll manually reset run_at.
	require.Eventually(t, func() bool {
		// Speed up retries by resetting run_at to now for failed jobs awaiting retry.
		failedJobs, _, _ := repo.ListPaged(context.Background(), burrow.PageRequest{Limit: 100, Page: 1}, StatusFailed)
		now := time.Now()
		for _, j := range failedJobs {
			j.RunAt = now
			_ = den.Update(context.Background(), db, j)
		}
		return attempts.Load() >= 3
	}, 5*time.Second, 20*time.Millisecond)

	cancel()
	<-w.Done()

	// Verify the job completed.
	allJobs, _, listErr := repo.ListPaged(context.Background(), burrow.PageRequest{Limit: 1, Page: 1}, "")
	require.NoError(t, listErr)
	require.Len(t, allJobs, 1)
	assert.Equal(t, StatusCompleted, allJobs[0].Status)
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
	w := NewWorker(repo, handlers, cfg, nil)

	// Enqueue with maxRetries=1 (only 1 attempt allowed).
	_, err := repo.Enqueue(context.Background(), "always_fail", `{}`, 1, time.Now())
	require.NoError(t, err)

	go w.Start(ctx)

	require.Eventually(t, func() bool {
		deadJobs, _, _ := repo.ListPaged(context.Background(), burrow.PageRequest{Limit: 1, Page: 1}, StatusDead)
		return len(deadJobs) > 0
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
	w := NewWorker(repo, handlers, cfg, nil)

	_, err := repo.Enqueue(context.Background(), "nonexistent", `{}`, 3, time.Now())
	require.NoError(t, err)

	go w.Start(ctx)

	require.Eventually(t, func() bool {
		deadJobs, _, _ := repo.ListPaged(context.Background(), burrow.PageRequest{Limit: 1, Page: 1}, StatusDead)
		return len(deadJobs) > 0
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
	w := NewWorker(repo, handlers, cfg, nil)

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
	allJobs, _, listErr := repo.ListPaged(context.Background(), burrow.PageRequest{Limit: 1, Page: 1}, "")
	require.NoError(t, listErr)
	require.Len(t, allJobs, 1)
	assert.Equal(t, StatusCompleted, allJobs[0].Status)
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

	w := NewWorker(repo, handlers, cfg, nil)
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
	w := NewWorker(repo, handlers, cfg, nil)

	// Create a stale running job (locked 30 min ago).
	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)
	claimed, err := repo.Claim(ctx, 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	staleJob, err := den.FindByID[Job](ctx, db, job.ID)
	require.NoError(t, err)
	backdated := time.Now().Add(-30 * time.Minute)
	staleJob.LockedAt = &backdated
	require.NoError(t, den.Update(ctx, db, staleJob))

	// Create a completed job older than 24h.
	job2, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)
	require.NoError(t, repo.Complete(ctx, job2.ID))
	completedJob, err := den.FindByID[Job](ctx, db, job2.ID)
	require.NoError(t, err)
	oldCompleted := time.Now().Add(-48 * time.Hour)
	completedJob.CompletedAt = &oldCompleted
	require.NoError(t, den.Update(ctx, db, completedJob))

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

func TestWorker_Maintenance_NoStaleNoCompleted(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	handlers := map[string]burrow.JobHandlerFunc{}
	cfg := testWorkerConfig()
	w := NewWorker(repo, handlers, cfg, nil)

	// Run maintenance with no stale or completed jobs — exercises the n==0 branches.
	w.maintenance(ctx)
}

func TestWorker_ProcessJob_Direct(t *testing.T) {
	tests := []struct { //nolint:govet // fieldalignment: test struct readability over optimization
		name       string
		typeName   string
		handler    burrow.JobHandlerFunc
		maxRetries int
		wantStatus JobStatus
		wantError  string
	}{
		{
			name:     "success completes job",
			typeName: "ok",
			handler: func(_ context.Context, _ []byte) error {
				return nil
			},
			maxRetries: 3,
			wantStatus: StatusCompleted,
		},
		{
			name:     "handler failure with retries left marks failed",
			typeName: "fail_retry",
			handler: func(_ context.Context, _ []byte) error {
				return fmt.Errorf("handler error")
			},
			maxRetries: 3,
			wantStatus: StatusFailed,
			wantError:  "handler error",
		},
		{
			name:     "handler failure at max retries marks dead",
			typeName: "fail_dead",
			handler: func(_ context.Context, _ []byte) error {
				return fmt.Errorf("permanent")
			},
			maxRetries: 1,
			wantStatus: StatusDead,
			wantError:  "permanent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testDB(t)
			repo := NewRepository(db)

			handlers := map[string]burrow.JobHandlerFunc{tt.typeName: tt.handler}
			cfg := testWorkerConfig()
			cfg.RetryBaseDelay = time.Millisecond
			w := NewWorker(repo, handlers, cfg, nil)

			_, err := repo.Enqueue(context.Background(), tt.typeName, `{}`, tt.maxRetries, time.Now())
			require.NoError(t, err)
			claimed, err := repo.Claim(context.Background(), 1)
			require.NoError(t, err)
			require.Len(t, claimed, 1)

			w.processJob(claimed[0])

			job, err := repo.GetByID(context.Background(), claimed[0].ID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, job.Status)
			if tt.wantError != "" {
				assert.Equal(t, tt.wantError, job.LastError)
			}
		})
	}

	t.Run("unknown type marks dead", func(t *testing.T) {
		db := testDB(t)
		repo := NewRepository(db)

		handlers := map[string]burrow.JobHandlerFunc{} // no handlers
		cfg := testWorkerConfig()
		w := NewWorker(repo, handlers, cfg, nil)

		_, err := repo.Enqueue(context.Background(), "unknown", `{}`, 3, time.Now())
		require.NoError(t, err)
		claimed, err := repo.Claim(context.Background(), 1)
		require.NoError(t, err)
		require.Len(t, claimed, 1)

		w.processJob(claimed[0])

		job, err := repo.GetByID(context.Background(), claimed[0].ID)
		require.NoError(t, err)
		assert.Equal(t, StatusDead, job.Status)
		assert.Contains(t, job.LastError, "unknown job type")
	})
}

func TestWorker_InjectsTemplateExecutor(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	gotExec := make(chan bool, 1)
	handlers := map[string]burrow.JobHandlerFunc{
		"check_exec": func(ctx context.Context, _ []byte) error {
			gotExec <- burrow.TemplateExec(ctx) != nil
			return nil
		},
	}

	exec := burrow.TemplateExecutor(func(_ context.Context, _ string, _ map[string]any) (template.HTML, error) {
		return "test", nil
	})
	cfg := testWorkerConfig()
	w := NewWorker(repo, handlers, cfg, exec)

	_, err := repo.Enqueue(context.Background(), "check_exec", `{}`, 3, time.Now())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	select {
	case hasExec := <-gotExec:
		assert.True(t, hasExec, "job handler should have TemplateExecutor in context")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for job handler")
	}

	cancel()
	<-w.Done()
}

func TestWorker_NoTemplateExecutor(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	gotExec := make(chan bool, 1)
	handlers := map[string]burrow.JobHandlerFunc{
		"check_no_exec": func(ctx context.Context, _ []byte) error {
			gotExec <- burrow.TemplateExec(ctx) != nil
			return nil
		},
	}

	cfg := testWorkerConfig()
	w := NewWorker(repo, handlers, cfg, nil)
	// templateExec is nil — not set.

	_, err := repo.Enqueue(context.Background(), "check_no_exec", `{}`, 3, time.Now())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	select {
	case hasExec := <-gotExec:
		assert.False(t, hasExec, "job handler should NOT have TemplateExecutor when not set")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for job handler")
	}

	cancel()
	<-w.Done()
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
	w := NewWorker(repo, handlers, cfg, nil)

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
