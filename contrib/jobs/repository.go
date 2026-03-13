package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/oliverandrich/burrow"
	"github.com/uptrace/bun"
)

// Sentinel errors for admin operations.
var (
	ErrNotFound      = sql.ErrNoRows
	ErrInvalidStatus = errors.New("invalid job status for this operation")
)

// Repository provides data access for the jobs queue.
type Repository struct {
	db *bun.DB
}

// NewRepository creates a new jobs Repository.
func NewRepository(db *bun.DB) *Repository {
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
	if _, err := r.db.NewInsert().Model(job).Exec(ctx); err != nil {
		return nil, fmt.Errorf("enqueue job %q: %w", typeName, err)
	}
	return job, nil
}

// Claim atomically claims up to limit pending or failed jobs that are ready to run.
// It sets their status to running and locked_at to the current time.
func (r *Repository) Claim(ctx context.Context, limit int) ([]Job, error) {
	now := time.Now()
	var jobs []Job
	if err := r.db.NewRaw(
		"UPDATE _jobs SET status = ?, locked_at = ?, attempts = attempts + 1, updated_at = ? "+
			"WHERE id IN (SELECT id FROM _jobs WHERE status IN (?, ?) AND run_at <= ? ORDER BY run_at ASC LIMIT ?) "+
			"RETURNING *",
		StatusRunning, now, now,
		StatusPending, StatusFailed, now, limit,
	).Scan(ctx, &jobs); err != nil {
		return nil, fmt.Errorf("claim jobs: %w", err)
	}
	return jobs, nil
}

// Complete marks a job as completed.
func (r *Repository) Complete(ctx context.Context, jobID int64) error {
	now := time.Now()
	if _, err := r.db.NewUpdate().Model((*Job)(nil)).
		Set("status = ?", StatusCompleted).
		Set("completed_at = ?", now).
		Set("updated_at = ?", now).
		Where("id = ?", jobID).
		Exec(ctx); err != nil {
		return fmt.Errorf("complete job %d: %w", jobID, err)
	}
	return nil
}

// Fail records a job failure. If attempts < maxRetries, the job is re-queued
// with exponential backoff (baseDelay * 2^(attempts-1)). Otherwise it is marked dead.
func (r *Repository) Fail(ctx context.Context, jobID int64, errMsg string, attempts, maxRetries int, baseDelay time.Duration) error {
	now := time.Now()
	if attempts < maxRetries {
		backoff := baseDelay * time.Duration(math.Pow(2, float64(attempts-1)))
		runAt := now.Add(backoff)
		if _, err := r.db.NewUpdate().Model((*Job)(nil)).
			Set("status = ?", StatusFailed).
			Set("last_error = ?", errMsg).
			Set("run_at = ?", runAt).
			Set("locked_at = NULL").
			Set("updated_at = ?", now).
			Where("id = ?", jobID).
			Exec(ctx); err != nil {
			return fmt.Errorf("fail job %d (retry): %w", jobID, err)
		}
		return nil
	}

	if _, err := r.db.NewUpdate().Model((*Job)(nil)).
		Set("status = ?", StatusDead).
		Set("last_error = ?", errMsg).
		Set("failed_at = ?", now).
		Set("locked_at = NULL").
		Set("updated_at = ?", now).
		Where("id = ?", jobID).
		Exec(ctx); err != nil {
		return fmt.Errorf("fail job %d (dead): %w", jobID, err)
	}
	return nil
}

// DeleteCompleted removes completed jobs older than the given duration.
func (r *Repository) DeleteCompleted(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	res, err := r.db.NewDelete().Model((*Job)(nil)).
		Where("status = ? AND completed_at < ?", StatusCompleted, cutoff).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete completed jobs: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// GetByID returns a single job by ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*Job, error) {
	job := new(Job)
	if err := r.db.NewSelect().Model(job).Where("id = ?", id).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get job %d: %w", id, err)
	}
	return job, nil
}

// ListPaged returns a paginated list of jobs, optionally filtered by status.
// Results are ordered by created_at DESC, id DESC.
func (r *Repository) ListPaged(ctx context.Context, pr burrow.PageRequest, status JobStatus) ([]Job, burrow.PageResult, error) {
	q := r.db.NewSelect().Model((*Job)(nil))
	if status != "" {
		q = q.Where("status = ?", status)
	}

	totalCount, err := q.Count(ctx)
	if err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("count jobs: %w", err)
	}

	var jobs []Job
	q = r.db.NewSelect().Model(&jobs).
		OrderExpr("created_at DESC, id DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q = burrow.ApplyOffset(q, pr)

	if err := q.Scan(ctx); err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("list jobs: %w", err)
	}

	return jobs, burrow.OffsetResult(pr, totalCount), nil
}

// Delete deletes a job by ID (any status).
func (r *Repository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.NewDelete().Model((*Job)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete job %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Retry resets a dead or failed job back to pending for re-processing.
func (r *Repository) Retry(ctx context.Context, id int64) error {
	job, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if job.Status != StatusFailed && job.Status != StatusDead {
		return ErrInvalidStatus
	}

	now := time.Now()
	_, err = r.db.NewUpdate().Model((*Job)(nil)).
		Set("status = ?", StatusPending).
		Set("attempts = 0").
		Set("last_error = ''").
		Set("failed_at = NULL").
		Set("locked_at = NULL").
		Set("run_at = ?", now).
		Set("updated_at = ?", now).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("retry job %d: %w", id, err)
	}
	return nil
}

// Cancel marks a pending, running, or failed job as dead.
func (r *Repository) Cancel(ctx context.Context, id int64) error {
	job, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if job.Status != StatusPending && job.Status != StatusRunning && job.Status != StatusFailed {
		return ErrInvalidStatus
	}

	now := time.Now()
	_, err = r.db.NewUpdate().Model((*Job)(nil)).
		Set("status = ?", StatusDead).
		Set("failed_at = ?", now).
		Set("locked_at = NULL").
		Set("updated_at = ?", now).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("cancel job %d: %w", id, err)
	}
	return nil
}

// RescueStale resets running jobs that have been locked longer than the
// given duration back to pending status.
func (r *Repository) RescueStale(ctx context.Context, staleDuration time.Duration) (int64, error) {
	cutoff := time.Now().Add(-staleDuration)
	now := time.Now()
	res, err := r.db.NewUpdate().Model((*Job)(nil)).
		Set("status = ?", StatusPending).
		Set("locked_at = NULL").
		Set("updated_at = ?", now).
		Where("status = ? AND locked_at < ?", StatusRunning, cutoff).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("rescue stale jobs: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
