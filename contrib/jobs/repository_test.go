package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/oliverandrich/den"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oliverandrich/burrow"
)

func testDB(t *testing.T) *den.DB {
	t.Helper()
	db := burrow.TestDB(t)

	err := den.Register(t.Context(), db, &Job{})
	require.NoError(t, err)
	return db
}

func TestRepository_Enqueue(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "send_email", `{"to":"user@example.com"}`, 3, time.Now())
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
		_, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now().Add(-time.Duration(3-i)*time.Second))
		require.NoError(t, err)
	}

	// Claim 2 — should get the 2 oldest.
	claimed, err := repo.Claim(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, claimed, 2)
	for _, j := range claimed {
		assert.Equal(t, StatusRunning, j.Status)
		assert.NotNil(t, j.LockedAt)
		// Attempts are incremented by the worker after claim, not during claim.
		assert.Equal(t, 0, j.Attempts)
	}

	// Claim again — should get the remaining 1.
	claimed2, err := repo.Claim(ctx, 5)
	require.NoError(t, err)
	assert.Len(t, claimed2, 1)
}

func TestRepository_Claim_RespectsRunAt(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Enqueue a future job.
	_, err := repo.Enqueue(ctx, "future", `{}`, 3, time.Now().Add(time.Hour))
	require.NoError(t, err)

	// Claim should return nothing.
	claimed, err := repo.Claim(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, claimed)
}

func TestRepository_Complete(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)

	err = repo.Complete(ctx, job)
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

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)

	// Fail with attempts=1, maxRetries=3 -> should retry.
	job.Attempts = 1
	job.MaxRetries = 3
	err = repo.Fail(ctx, job, "connection timeout", 30*time.Second)
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
	}

	for _, tt := range tests {
		job, err := repo.Enqueue(ctx, "task", `{}`, 10, time.Now())
		require.NoError(t, err)

		job.Attempts = tt.attempt
		job.MaxRetries = 10
		before := time.Now()
		err = repo.Fail(ctx, job, "err", baseDelay)
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

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)

	// Fail with attempts=3, maxRetries=3 -> should be dead.
	job.Attempts = 3
	job.MaxRetries = 3
	err = repo.Fail(ctx, job, "permanent failure", 30*time.Second)
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

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)

	// Complete and backdate.
	err = repo.Complete(ctx, job)
	require.NoError(t, err)

	// Backdate the completed_at time.
	updated, err := den.FindByID[Job](ctx, db, job.ID)
	require.NoError(t, err)
	backdated := time.Now().Add(-2 * time.Hour)
	updated.CompletedAt = &backdated
	err = den.Update(ctx, db, updated)
	require.NoError(t, err)

	// Delete completed older than 1 hour.
	n, err := repo.DeleteCompleted(ctx, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// Verify it's gone.
	count, err := den.NewQuery[Job](ctx, db).Count()
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestRepository_RescueStale(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
	require.NoError(t, err)

	// Claim it, then backdate locked_at.
	claimed, err := repo.Claim(ctx, 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	// Backdate the locked_at time.
	updated, err := den.FindByID[Job](ctx, db, job.ID)
	require.NoError(t, err)
	backdated := time.Now().Add(-30 * time.Minute)
	updated.LockedAt = &backdated
	err = den.Update(ctx, db, updated)
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

	job, err := repo.Enqueue(ctx, "send_email", `{"to":"a@b.com"}`, 3, time.Now())
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
		j, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now().Add(-time.Duration(5-i)*time.Second))
		require.NoError(t, err)
		if i >= 3 {
			require.NoError(t, repo.Complete(ctx, j))
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

	job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
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
		job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
		require.NoError(t, err)
		job.Attempts = 3
		job.MaxRetries = 3
		require.NoError(t, repo.Fail(ctx, job, "boom", 30*time.Second)) // marks dead

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
		job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
		require.NoError(t, err)
		job.Attempts = 1
		job.MaxRetries = 3
		require.NoError(t, repo.Fail(ctx, job, "oops", 30*time.Second)) // marks failed

		err = repo.Retry(ctx, job.ID)
		require.NoError(t, err)

		got, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, StatusPending, got.Status)
	})

	t.Run("invalid status", func(t *testing.T) {
		job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
		require.NoError(t, err) // pending

		err = repo.Retry(ctx, job.ID)
		require.ErrorIs(t, err, ErrInvalidStatus)
	})

	t.Run("not found", func(t *testing.T) {
		err := repo.Retry(ctx, "nonexistent")
		require.ErrorIs(t, err, ErrNotFound)
	})
}

func TestRepository_Cancel(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("from pending", func(t *testing.T) {
		job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
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
		job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
		require.NoError(t, err)
		claimed, err := repo.Claim(ctx, 1)
		require.NoError(t, err)
		require.Len(t, claimed, 1)

		err = repo.Cancel(ctx, job.ID)
		require.NoError(t, err)

		got, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, StatusDead, got.Status)
	})

	t.Run("invalid status — completed", func(t *testing.T) {
		job, err := repo.Enqueue(ctx, "task", `{}`, 3, time.Now())
		require.NoError(t, err)
		require.NoError(t, repo.Complete(ctx, job))

		err = repo.Cancel(ctx, job.ID)
		require.ErrorIs(t, err, ErrInvalidStatus)
	})

	t.Run("not found", func(t *testing.T) {
		err := repo.Cancel(ctx, "nonexistent")
		require.ErrorIs(t, err, ErrNotFound)
	})
}
