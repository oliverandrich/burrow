package jobs

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/where"
)

// Sentinel errors for admin operations.
var (
	ErrNotFound      = den.ErrNotFound
	ErrInvalidStatus = errors.New("invalid job status for this operation")
)

// Repository provides data access for the jobs queue.
type Repository struct {
	db *den.DB
}

// NewRepository creates a new jobs Repository.
func NewRepository(db *den.DB) *Repository {
	return &Repository{db: db}
}

// Enqueue inserts a new job into the queue.
func (r *Repository) Enqueue(ctx context.Context, typeName, payload string, maxRetries int, runAt time.Time) (*Job, error) {
	job := &Job{
		Type:       typeName,
		Payload:    payload,
		Status:     StatusPending,
		MaxRetries: maxRetries,
		RunAt:      runAt,
	}
	if err := den.Insert(ctx, r.db, job); err != nil {
		return nil, fmt.Errorf("enqueue job %q: %w", typeName, err)
	}
	return job, nil
}

// Claim atomically claims up to limit pending or failed jobs that are ready to run.
// Uses a transaction to find eligible jobs, update their status, and increment attempts atomically.
func (r *Repository) Claim(ctx context.Context, limit int) ([]*Job, error) {
	var claimed []*Job
	now := time.Now()
	nowStr := now.Format(time.RFC3339Nano)

	err := den.RunInTransaction(ctx, r.db, func(tx *den.Tx) error {
		eligible, err := den.NewQuery[Job](ctx, r.db,
			where.Field("status").In(string(StatusPending), string(StatusFailed)),
			where.Field("run_at").Lte(nowStr),
		).Sort("run_at", den.Asc).Limit(limit).All()
		if err != nil {
			return fmt.Errorf("query eligible jobs: %w", err)
		}

		for _, job := range eligible {
			job.Status = StatusRunning
			job.LockedAt = &now
			job.Attempts++
			if err := den.TxUpdate(tx, job); err != nil {
				return fmt.Errorf("claim job %s: %w", job.ID, err)
			}
			claimed = append(claimed, job)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("claim jobs: %w", err)
	}
	return claimed, nil
}

// Complete marks a job as completed.
func (r *Repository) Complete(ctx context.Context, jobID string) error {
	job, err := den.FindByID[Job](ctx, r.db, jobID)
	if err != nil {
		return fmt.Errorf("complete job %s: %w", jobID, err)
	}
	now := time.Now()
	job.Status = StatusCompleted
	job.CompletedAt = &now
	if err := den.Update(ctx, r.db, job); err != nil {
		return fmt.Errorf("complete job %s: %w", jobID, err)
	}
	return nil
}

// Fail records a job failure. If attempts < maxRetries, the job is re-queued
// with exponential backoff (baseDelay * 2^(attempts-1)). Otherwise it is marked dead.
func (r *Repository) Fail(ctx context.Context, jobID string, errMsg string, attempts, maxRetries int, baseDelay time.Duration) error {
	job, err := den.FindByID[Job](ctx, r.db, jobID)
	if err != nil {
		return fmt.Errorf("fail job %s: %w", jobID, err)
	}
	now := time.Now()

	if attempts < maxRetries {
		backoff := baseDelay * time.Duration(math.Pow(2, float64(attempts-1)))
		job.Status = StatusFailed
		job.LastError = errMsg
		job.RunAt = now.Add(backoff)
		job.LockedAt = nil
	} else {
		job.Status = StatusDead
		job.LastError = errMsg
		job.FailedAt = &now
		job.LockedAt = nil
	}

	if err := den.Update(ctx, r.db, job); err != nil {
		return fmt.Errorf("fail job %s: %w", jobID, err)
	}
	return nil
}

// DeleteCompleted removes completed jobs older than the given duration.
func (r *Repository) DeleteCompleted(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoffStr := time.Now().Add(-olderThan).Format(time.RFC3339Nano)
	return den.DeleteMany[Job](ctx, r.db, []where.Condition{
		where.Field("status").Eq(string(StatusCompleted)),
		where.Field("completed_at").Lt(cutoffStr),
	})
}

// GetByID returns a single job by ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Job, error) {
	return den.FindByID[Job](ctx, r.db, id)
}

// ListPaged returns a paginated list of jobs, optionally filtered by status.
// Results are ordered by created_at DESC.
func (r *Repository) ListPaged(ctx context.Context, pr burrow.PageRequest, status JobStatus) ([]*Job, burrow.PageResult, error) {
	qs := den.NewQuery[Job](ctx, r.db)
	if status != "" {
		qs = qs.Where(where.Field("status").Eq(string(status)))
	}
	qs = qs.Sort("_created_at", den.Desc).Limit(pr.Limit).Skip(pr.Offset())

	jobs, count, err := qs.AllWithCount()
	if err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("list jobs: %w", err)
	}

	return jobs, burrow.OffsetResult(pr, int(count)), nil
}

// Delete deletes a job by ID (any status).
func (r *Repository) Delete(ctx context.Context, id string) error {
	job, err := den.FindByID[Job](ctx, r.db, id)
	if err != nil {
		return err
	}
	return den.Delete(ctx, r.db, job)
}

// Retry resets a dead or failed job back to pending for re-processing.
func (r *Repository) Retry(ctx context.Context, id string) error {
	now := time.Now()
	_, err := den.FindOneAndUpdate[Job](ctx, r.db,
		den.SetFields{
			"status":     string(StatusPending),
			"attempts":   0,
			"last_error": "",
			"failed_at":  nil,
			"locked_at":  nil,
			"run_at":     now,
		},
		where.Field("_id").Eq(id),
		where.Field("status").In(string(StatusFailed), string(StatusDead)),
	)
	if err != nil {
		if errors.Is(err, den.ErrNotFound) {
			// Either doesn't exist or wrong status
			if _, getErr := r.GetByID(ctx, id); getErr != nil {
				return getErr
			}
			return ErrInvalidStatus
		}
		return fmt.Errorf("retry job %s: %w", id, err)
	}
	return nil
}

// Cancel marks a pending, running, or failed job as dead.
func (r *Repository) Cancel(ctx context.Context, id string) error {
	now := time.Now()
	_, err := den.FindOneAndUpdate[Job](ctx, r.db,
		den.SetFields{
			"status":    string(StatusDead),
			"failed_at": &now,
			"locked_at": nil,
		},
		where.Field("_id").Eq(id),
		where.Field("status").In(string(StatusPending), string(StatusRunning), string(StatusFailed)),
	)
	if err != nil {
		if errors.Is(err, den.ErrNotFound) {
			if _, getErr := r.GetByID(ctx, id); getErr != nil {
				return getErr
			}
			return ErrInvalidStatus
		}
		return fmt.Errorf("cancel job %s: %w", id, err)
	}
	return nil
}

// RescueStale resets running jobs that have been locked longer than the
// given duration back to pending status.
func (r *Repository) RescueStale(ctx context.Context, staleDuration time.Duration) (int64, error) {
	cutoffStr := time.Now().Add(-staleDuration).Format(time.RFC3339Nano)

	stale, err := den.NewQuery[Job](ctx, r.db,
		where.Field("status").Eq(string(StatusRunning)),
		where.Field("locked_at").Lt(cutoffStr),
	).All()
	if err != nil {
		return 0, fmt.Errorf("rescue stale jobs: %w", err)
	}

	var count int64
	for _, job := range stale {
		job.Status = StatusPending
		job.LockedAt = nil
		if err := den.Update(ctx, r.db, job); err != nil {
			return count, fmt.Errorf("rescue stale job %s: %w", job.ID, err)
		}
		count++
	}
	return count, nil
}
