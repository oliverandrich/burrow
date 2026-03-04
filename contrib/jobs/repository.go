package jobs

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/uptrace/bun"
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
// with exponential backoff (2^attempts seconds). Otherwise it is marked dead.
func (r *Repository) Fail(ctx context.Context, jobID int64, errMsg string, attempts, maxRetries int) error {
	now := time.Now()
	if attempts < maxRetries {
		backoff := time.Duration(math.Pow(2, float64(attempts))) * time.Second
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
		ForceDelete().
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete completed jobs: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
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
