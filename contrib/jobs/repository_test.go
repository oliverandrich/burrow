package jobs

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/where"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/burrowtest"
)

func testDB(t *testing.T) *den.DB {
	t.Helper()
	db := burrowtest.DB(t)

	err := den.Register(t.Context(), db, &Job{})
	require.NoError(t, err)
	return db
}

func TestRepository_Enqueue(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "send_email", `{"to":"user@example.com"}`, 3, 0, time.Now())
	require.NoError(t, err)
	assert.NotEmpty(t, job.ID)
	assert.Equal(t, "send_email", job.Type)
	assert.JSONEq(t, `{"to":"user@example.com"}`, job.Payload)
	assert.Equal(t, StatusPending, job.Status)
	assert.Equal(t, 3, job.MaxRetries)
}

func TestRepository_Claim(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Enqueue 3 jobs.
	for i := range 3 {
		_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now().Add(-time.Duration(3-i)*time.Second))
		require.NoError(t, err)
	}

	// Claim 2 — should get the 2 oldest.
	claimed, err := repo.Claim(ctx, "test-worker", 2)
	require.NoError(t, err)
	assert.Len(t, claimed, 2)
	for _, j := range claimed {
		assert.Equal(t, StatusRunning, j.Status)
		assert.NotNil(t, j.LockedAt)
		// Attempts are incremented by the worker after claim, not during claim.
		assert.Equal(t, 0, j.Attempts)
	}

	// Claim again — should get the remaining 1.
	claimed2, err := repo.Claim(ctx, "test-worker", 5)
	require.NoError(t, err)
	assert.Len(t, claimed2, 1)
}

func TestRepository_Claim_RespectsRunAt(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Enqueue a future job.
	_, err := repo.Enqueue(ctx, "future", `{}`, 3, 0, time.Now().Add(time.Hour))
	require.NoError(t, err)

	// Claim should return nothing.
	claimed, err := repo.Claim(ctx, "test-worker", 10)
	require.NoError(t, err)
	assert.Empty(t, claimed)
}

func TestRepository_Complete(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	claimed, err := repo.Claim(ctx, "test-worker", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	job := claimed[0]
	job.Attempts = 1

	err = repo.Complete(ctx, job, "")
	require.NoError(t, err)

	// Verify status.
	updated, err := den.FindByID[Job](ctx, db, job.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, updated.Status)
	assert.NotNil(t, updated.CompletedAt)
}

func TestRepository_Fail_Retry(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	claimed, err := repo.Claim(ctx, "test-worker", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	job := claimed[0]

	// Fail with attempts=1, maxRetries=3 -> should retry.
	job.Attempts = 1
	job.MaxRetries = 3
	err = repo.Fail(ctx, job, "connection timeout", "", 30*time.Second)
	require.NoError(t, err)

	updated, err := den.FindByID[Job](ctx, db, job.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, updated.Status)
	assert.Equal(t, "connection timeout", updated.LastError)
	assert.True(t, updated.RunAt.After(time.Now()), "run_at should be in the future")
}

func TestRepository_Fail_BackoffDuration(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	baseDelay := 30 * time.Second

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 30 * time.Second},  // 30s * 2^0
		{2, 60 * time.Second},  // 30s * 2^1
		{3, 120 * time.Second}, // 30s * 2^2
		{4, 240 * time.Second}, // 30s * 2^3
		{20, time.Hour},        // 30s * 2^19 would be ~182 days, capped to 1h
		{30, time.Hour},        // 30s * 2^29 would be ~512 years, capped to 1h
	}

	for _, tt := range tests {
		_, err := repo.Enqueue(ctx, "task", `{}`, 50, 0, time.Now())
		require.NoError(t, err)

		claimed, err := repo.Claim(ctx, "test-worker", 1)
		require.NoError(t, err)
		require.Len(t, claimed, 1)
		job := claimed[0]

		job.Attempts = tt.attempt
		job.MaxRetries = 50
		before := time.Now()
		err = repo.Fail(ctx, job, "err", "", baseDelay)
		require.NoError(t, err)

		updated, err := den.FindByID[Job](ctx, db, job.ID)
		require.NoError(t, err)

		expectedRunAt := before.Add(tt.expected)
		assert.InDelta(t, expectedRunAt.Unix(), updated.RunAt.Unix(), 2,
			"attempt %d: expected ~%v backoff", tt.attempt, tt.expected)
	}
}

func TestRepository_Fail_Dead(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	claimed, err := repo.Claim(ctx, "test-worker", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	job := claimed[0]

	// Fail with attempts=3, maxRetries=3 -> should be dead.
	job.Attempts = 3
	job.MaxRetries = 3
	err = repo.Fail(ctx, job, "permanent failure", "", 30*time.Second)
	require.NoError(t, err)

	updated, err := den.FindByID[Job](ctx, db, job.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusDead, updated.Status)
	assert.Equal(t, "permanent failure", updated.LastError)
	assert.NotNil(t, updated.FailedAt)
}

func TestRepository_DeleteCompleted(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	claimed, err := repo.Claim(ctx, "test-worker", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	job := claimed[0]
	job.Attempts = 1

	// Complete and backdate.
	err = repo.Complete(ctx, job, "")
	require.NoError(t, err)

	// Backdate the completed_at time.
	updated, err := den.FindByID[Job](ctx, db, job.ID)
	require.NoError(t, err)
	backdated := time.Now().Add(-2 * time.Hour)
	updated.CompletedAt = &backdated
	err = den.Save(ctx, db, updated)
	require.NoError(t, err)

	// Delete completed older than 1 hour.
	n, err := repo.DeleteCompleted(ctx, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// Verify it's gone.
	count, err := den.NewQuery[Job](db).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestRepository_RescueStale(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	// Claim it, then backdate locked_at.
	claimed, err := repo.Claim(ctx, "test-worker", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	// Backdate the locked_at time.
	updated, err := den.FindByID[Job](ctx, db, job.ID)
	require.NoError(t, err)
	backdated := time.Now().Add(-30 * time.Minute)
	updated.LockedAt = &backdated
	err = den.Save(ctx, db, updated)
	require.NoError(t, err)

	// Rescue stale jobs older than 10 minutes.
	n, err := repo.RescueStale(ctx, 10*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// Verify reset to pending.
	rescued, err := den.FindByID[Job](ctx, db, job.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, rescued.Status)
	assert.Nil(t, rescued.LockedAt)
}

func TestRepository_GetByID(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "send_email", `{"to":"a@b.com"}`, 3, 0, time.Now())
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, job.ID, got.ID)
	assert.Equal(t, "send_email", got.Type)

	// Not found.
	_, err = repo.GetByID(ctx, "nonexistent")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestRepository_ListPaged(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create jobs with different statuses.
	for i := range 5 {
		_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now().Add(-time.Duration(5-i)*time.Second))
		require.NoError(t, err)
		if i >= 3 {
			claimed, claimErr := repo.Claim(ctx, "test-worker", 1)
			require.NoError(t, claimErr)
			require.Len(t, claimed, 1)
			claimed[0].Attempts = 1
			require.NoError(t, repo.Complete(ctx, claimed[0], ""))
		}
	}

	// List all (no status filter).
	jobs, page, err := repo.ListPaged(ctx, burrow.PageRequest{Limit: 10, Page: 1}, "")
	require.NoError(t, err)
	assert.Len(t, jobs, 5)
	assert.Equal(t, 5, page.TotalCount)

	// Filter by pending.
	jobs, page, err = repo.ListPaged(ctx, burrow.PageRequest{Limit: 10, Page: 1}, StatusPending)
	require.NoError(t, err)
	assert.Len(t, jobs, 3)
	assert.Equal(t, 3, page.TotalCount)

	// Pagination: page 1 with limit 2.
	jobs, page, err = repo.ListPaged(ctx, burrow.PageRequest{Limit: 2, Page: 1}, "")
	require.NoError(t, err)
	assert.Len(t, jobs, 2)
	assert.Equal(t, 5, page.TotalCount)
	assert.Equal(t, 3, page.TotalPages)
	assert.True(t, page.HasMore)
}

func TestRepository_Delete(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	err = repo.Delete(ctx, job.ID)
	require.NoError(t, err)

	// Verify gone.
	_, err = repo.GetByID(ctx, job.ID)
	require.ErrorIs(t, err, ErrNotFound)

	// Delete non-existent — should return ErrNotFound.
	err = repo.Delete(ctx, "nonexistent")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestRepository_Retry(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("from dead", func(t *testing.T) {
		_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
		require.NoError(t, err)
		claimed, err := repo.Claim(ctx, "test-worker", 1)
		require.NoError(t, err)
		require.Len(t, claimed, 1)
		job := claimed[0]
		job.Attempts = 3
		job.MaxRetries = 3
		require.NoError(t, repo.Fail(ctx, job, "boom", "", 30*time.Second)) // marks dead

		err = repo.Retry(ctx, job.ID)
		require.NoError(t, err)

		got, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, StatusPending, got.Status)
		assert.Equal(t, 0, got.Attempts)
		assert.Empty(t, got.LastError)
		assert.Nil(t, got.FailedAt)
		assert.Nil(t, got.LockedAt)
	})

	t.Run("from failed", func(t *testing.T) {
		_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
		require.NoError(t, err)
		claimed, err := repo.Claim(ctx, "test-worker", 1)
		require.NoError(t, err)
		require.Len(t, claimed, 1)
		job := claimed[0]
		job.Attempts = 1
		job.MaxRetries = 3
		require.NoError(t, repo.Fail(ctx, job, "oops", "", 30*time.Second)) // marks failed

		err = repo.Retry(ctx, job.ID)
		require.NoError(t, err)

		got, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, StatusPending, got.Status)
	})

	t.Run("invalid status", func(t *testing.T) {
		job, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
		require.NoError(t, err) // pending

		err = repo.Retry(ctx, job.ID)
		require.ErrorIs(t, err, ErrInvalidStatus)
	})

	t.Run("not found", func(t *testing.T) {
		err := repo.Retry(ctx, "nonexistent")
		require.ErrorIs(t, err, ErrNotFound)
	})
}

func TestClaim_SetsWorkerID(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	claimed, err := repo.Claim(ctx, "worker-1", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, "worker-1", claimed[0].WorkerID)

	// Verify in the database too.
	got, err := repo.GetByID(ctx, claimed[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "worker-1", got.WorkerID)
}

func TestClaim_SkipsOwnedJobs(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create a failed job and manually set worker_id (simulating an owned job).
	job, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)
	job.Status = StatusFailed
	job.WorkerID = "other-worker"
	require.NoError(t, den.Save(ctx, db, job))

	// Claim should skip it because worker_id is not empty.
	claimed, err := repo.Claim(ctx, "worker-1", 1)
	require.NoError(t, err)
	assert.Empty(t, claimed)
}

func TestComplete_GuardRejectsWrongWorker(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	// Claim with worker A.
	claimed, err := repo.Claim(ctx, "worker-a", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	// Try to Complete with worker B's ID — should fail.
	claimed[0].WorkerID = "worker-b"
	claimed[0].Attempts = 1
	err = repo.Complete(ctx, claimed[0], "")
	require.ErrorIs(t, err, ErrStaleJob)

	// Job should still be running.
	got, err := repo.GetByID(ctx, claimed[0].ID)
	require.NoError(t, err)
	assert.Equal(t, StatusRunning, got.Status)
}

func TestFail_GuardRejectsWrongWorker(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	// Claim with worker A.
	claimed, err := repo.Claim(ctx, "worker-a", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	// Try to Fail with worker B's ID — should fail.
	claimed[0].WorkerID = "worker-b"
	claimed[0].Attempts = 1
	err = repo.Fail(ctx, claimed[0], "error", "", 30*time.Second)
	require.ErrorIs(t, err, ErrStaleJob)

	// Job should still be running.
	got, err := repo.GetByID(ctx, claimed[0].ID)
	require.NoError(t, err)
	assert.Equal(t, StatusRunning, got.Status)
}

func TestComplete_ClearsWorkerID(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	claimed, err := repo.Claim(ctx, "worker-1", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	claimed[0].Attempts = 1
	err = repo.Complete(ctx, claimed[0], "")
	require.NoError(t, err)

	// Worker ID should be cleared after completion.
	got, err := repo.GetByID(ctx, claimed[0].ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, got.Status)
	assert.Empty(t, got.WorkerID)
}

func TestRepository_Cancel(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("from pending", func(t *testing.T) {
		job, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
		require.NoError(t, err)

		err = repo.Cancel(ctx, job.ID)
		require.NoError(t, err)

		got, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, StatusDead, got.Status)
		assert.NotNil(t, got.FailedAt)
		assert.Nil(t, got.LockedAt)
	})

	t.Run("from running", func(t *testing.T) {
		job, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
		require.NoError(t, err)
		claimed, err := repo.Claim(ctx, "test-worker", 1)
		require.NoError(t, err)
		require.Len(t, claimed, 1)

		err = repo.Cancel(ctx, job.ID)
		require.NoError(t, err)

		got, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, StatusDead, got.Status)
	})

	t.Run("invalid status — completed", func(t *testing.T) {
		_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
		require.NoError(t, err)
		claimed, claimErr := repo.Claim(ctx, "test-worker", 1)
		require.NoError(t, claimErr)
		require.Len(t, claimed, 1)
		job := claimed[0]
		require.NoError(t, repo.Complete(ctx, job, ""))

		err = repo.Cancel(ctx, job.ID)
		require.ErrorIs(t, err, ErrInvalidStatus)
	})

	t.Run("not found", func(t *testing.T) {
		err := repo.Cancel(ctx, "nonexistent")
		require.ErrorIs(t, err, ErrNotFound)
	})
}

func TestRepository_RescueStale_SkipsCompletedJobs(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create, claim, and complete a job.
	_, err := repo.Enqueue(ctx, "completed-task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	claimed, err := repo.Claim(ctx, "worker-1", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	job := claimed[0]

	// Backdate locked_at first (while still running).
	past := time.Now().Add(-10 * time.Minute)
	_, err = den.NewQuery[Job](db, where.Field("_id").Eq(job.ID)).
		UpdateOne(ctx, den.SetFields{"locked_at": &past})
	require.NoError(t, err)

	// Complete the job — simulates the race where worker finishes between
	// RescueStale's query and its update.
	job.Attempts = 1
	require.NoError(t, repo.Complete(ctx, job, ""))

	// RescueStale should skip the completed job.
	rescued, err := repo.RescueStale(ctx, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rescued, "should not reset a completed job")

	got, err := repo.GetByID(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, got.Status, "status should remain completed")
}

func TestRepository_Complete_WithResult(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	claimed, err := repo.Claim(ctx, "test-worker", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	job := claimed[0]
	job.Attempts = 1

	err = repo.Complete(ctx, job, `{"output":"done"}`)
	require.NoError(t, err)

	updated, err := den.FindByID[Job](ctx, db, job.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, updated.Status)
	assert.JSONEq(t, `{"output":"done"}`, updated.Result)
}

func TestRepository_Complete_WithoutResult(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	claimed, err := repo.Claim(ctx, "test-worker", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	job := claimed[0]
	job.Attempts = 1

	err = repo.Complete(ctx, job, "")
	require.NoError(t, err)

	updated, err := den.FindByID[Job](ctx, db, job.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, updated.Status)
	assert.Empty(t, updated.Result)
}

func TestRepository_Fail_ErrorClass(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	claimed, err := repo.Claim(ctx, "test-worker", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	job := claimed[0]
	job.Attempts = 3
	job.MaxRetries = 3

	err = repo.Fail(ctx, job, "connection refused", "*net.OpError", 30*time.Second)
	require.NoError(t, err)

	updated, err := den.FindByID[Job](ctx, db, job.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusDead, updated.Status)
	assert.Equal(t, "connection refused", updated.LastError)
	assert.Equal(t, "*net.OpError", updated.ErrorClass)
}

func TestRepository_GetResult(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	claimed, err := repo.Claim(ctx, "test-worker", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	job := claimed[0]
	job.Attempts = 1

	err = repo.Complete(ctx, job, `{"count":42}`)
	require.NoError(t, err)

	result, err := repo.GetResult(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, `{"count":42}`, result)
}

func TestRepository_GetResult_NotFound(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	_, err := repo.GetResult(context.Background(), "nonexistent")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestRepository_Retry_ClearsResultAndErrorClass(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
	require.NoError(t, err)

	claimed, err := repo.Claim(ctx, "test-worker", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	job := claimed[0]
	job.Attempts = 3
	job.MaxRetries = 3

	// Fail with error class and result won't be set (only on success), but
	// let's manually set a result to verify it gets cleared on retry.
	require.NoError(t, repo.Fail(ctx, job, "boom", "*errors.errorString", 30*time.Second))

	// Manually set result to verify retry clears it.
	_, err = den.NewQuery[Job](db, where.Field("_id").Eq(job.ID)).
		UpdateOne(ctx, den.SetFields{"result": `{"stale":"data"}`})
	require.NoError(t, err)

	err = repo.Retry(ctx, job.ID)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, got.Status)
	assert.Empty(t, got.LastError)
	assert.Empty(t, got.ErrorClass)
	assert.Empty(t, got.Result)
}

func TestRepository_Enqueue_WithPriority(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "urgent", `{}`, 3, 10, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 10, job.Priority)

	// Verify persisted.
	got, err := den.FindByID[Job](ctx, db, job.ID)
	require.NoError(t, err)
	assert.Equal(t, 10, got.Priority)
}

func TestRepository_Claim_PriorityOrdering(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Enqueue: normal (oldest), high, normal.
	_, err := repo.Enqueue(ctx, "task", `{"n":"old-normal"}`, 3, 0, time.Now().Add(-3*time.Second))
	require.NoError(t, err)
	highJob, err := repo.Enqueue(ctx, "task", `{"n":"high"}`, 3, 10, time.Now().Add(-1*time.Second))
	require.NoError(t, err)
	_, err = repo.Enqueue(ctx, "task", `{"n":"new-normal"}`, 3, 0, time.Now())
	require.NoError(t, err)

	// Claim 1 — should get the high-priority job, not the oldest.
	claimed, err := repo.Claim(ctx, "test-worker", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, highJob.ID, claimed[0].ID)
}

func TestRepository_Claim_SamePriority_FIFO(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Enqueue 3 jobs at same priority, different times.
	oldest, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now().Add(-3*time.Second))
	require.NoError(t, err)
	_, err = repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now().Add(-2*time.Second))
	require.NoError(t, err)
	_, err = repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now().Add(-1*time.Second))
	require.NoError(t, err)

	// Claim 1 — should get the oldest (FIFO within same priority).
	claimed, err := repo.Claim(ctx, "test-worker", 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, oldest.ID, claimed[0].ID)
}

// TestRepository_Claim_ConcurrentWorkers proves the SKIP LOCKED claim path:
// N workers racing to claim from the same pool end up with disjoint slices
// and no job is claimed twice. On SQLite the transaction serialization gives
// the same guarantee; on PostgreSQL the rows are partitioned across workers
// via `FOR UPDATE SKIP LOCKED`.
func TestRepository_Claim_ConcurrentWorkers(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	const (
		totalJobs  = 50
		numWorkers = 5
		perWorker  = 10
	)

	for range totalJobs {
		_, err := repo.Enqueue(ctx, "task", `{}`, 3, 0, time.Now())
		require.NoError(t, err)
	}

	type result struct {
		workerID string
		jobs     []*Job
		err      error
	}

	resultsCh := make(chan result, numWorkers)
	for i := range numWorkers {
		workerID := fmt.Sprintf("worker-%d", i)
		go func() {
			jobs, err := repo.Claim(ctx, workerID, perWorker)
			resultsCh <- result{workerID: workerID, jobs: jobs, err: err}
		}()
	}

	seen := make(map[string]string) // jobID -> workerID
	totalClaimed := 0
	for range numWorkers {
		r := <-resultsCh
		require.NoError(t, r.err)
		for _, j := range r.jobs {
			if prev, dup := seen[j.ID]; dup {
				t.Fatalf("job %s double-claimed by %s and %s", j.ID, prev, r.workerID)
			}
			seen[j.ID] = r.workerID
			assert.Equal(t, r.workerID, j.WorkerID, "worker_id on returned job must match the claiming worker")
			assert.Equal(t, StatusRunning, j.Status)
		}
		totalClaimed += len(r.jobs)
	}

	assert.Equal(t, totalJobs, totalClaimed, "all jobs should be claimed across all workers with no overlap")
	assert.Len(t, seen, totalJobs)
}
