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
func (r *Repository) Enqueue(ctx context.Context, typeName, payload string, maxRetries, priority int, runAt time.Time) (*Job, error) {
	job := &Job{
		Type:       typeName,
		Payload:    payload,
		Status:     StatusPending,
		Priority:   priority,
		MaxRetries: maxRetries,
		RunAt:      runAt,
	}
	if err := den.Insert(ctx, r.db, job); err != nil {
		return nil, fmt.Errorf("enqueue job %q: %w", typeName, err)
	}
	return job, nil
}

// Claim atomically claims up to limit pending or failed jobs that are ready to run.
// Jobs are claimed in priority order (highest first), with FIFO ordering within
// the same priority level. Each claim is a two-step operation: first find the
// best candidate via a sorted query, then atomically update it. If another
// worker claims the candidate between the two steps, we retry with the next one.
func (r *Repository) Claim(ctx context.Context, workerID string, limit int) ([]*Job, error) {
	var claimed []*Job
	now := time.Now()
	nowStr := now.Format(time.RFC3339Nano)

	readyConds := []where.Condition{
		where.Field("status").In(string(StatusPending), string(StatusFailed)),
		where.Field("run_at").Lte(nowStr),
		where.Field("worker_id").Eq(""),
	}

	for range limit {
		// Step 1: Find the highest-priority ready job.
		candidate, err := den.NewQuery[Job](ctx, r.db, readyConds...).
			Sort("priority", den.Desc).
			Sort("run_at", den.Asc).
			First()
		if err != nil {
			if errors.Is(err, den.ErrNotFound) {
				break
			}
			return nil, fmt.Errorf("claim jobs: find candidate: %w", err)
		}

		// Step 2: Atomically claim by ID + guards.
		job, err := den.FindOneAndUpdate[Job](ctx, r.db,
			den.SetFields{
				"status":    string(StatusRunning),
				"locked_at": &now,
				"worker_id": workerID,
			},
			where.Field("_id").Eq(candidate.ID),
			where.Field("status").In(string(StatusPending), string(StatusFailed)),
			where.Field("worker_id").Eq(""),
		)
		if err != nil {
			if errors.Is(err, den.ErrNotFound) {
				continue // Lost race — another worker claimed it, try next.
			}
			return nil, fmt.Errorf("claim jobs: update: %w", err)
		}
		claimed = append(claimed, job)
	}
	return claimed, nil
}

// Complete marks a job as completed. The result parameter holds an optional
// JSON-encoded handler return value (empty string means no result). The update
// is guarded by an ownership check: only the worker that claimed the job
// (matching worker_id and status=running) can complete it. Returns ErrStaleJob
// if ownership has changed.
func (r *Repository) Complete(ctx context.Context, job *Job, result string) error {
	now := time.Now()
	_, err := den.FindOneAndUpdate[Job](ctx, r.db,
		den.SetFields{
			"status":            string(StatusCompleted),
			"completed_at":      &now,
			"attempts":          job.Attempts,
			"result":            result,
			"last_attempted_at": job.LastAttemptedAt,
			"worker_id":         "",
		},
		where.Field("_id").Eq(job.ID),
		where.Field("status").Eq(string(StatusRunning)),
		where.Field("worker_id").Eq(job.WorkerID),
	)
	if errors.Is(err, den.ErrNotFound) {
		return ErrStaleJob
	}
	if err != nil {
		return fmt.Errorf("complete job %s: %w", job.ID, err)
	}
	return nil
}

// Fail records a job failure. If attempts < maxRetries, the job is re-queued
// with exponential backoff (baseDelay * 2^(attempts-1)). Otherwise it is marked dead.
// The errorClass parameter stores the Go type name of the error for monitoring.
// The update is guarded by an ownership check matching worker_id and status=running.
// Returns ErrStaleJob if ownership has changed.
func (r *Repository) Fail(ctx context.Context, job *Job, errMsg, errorClass string, baseDelay time.Duration) error {
	now := time.Now()
	fields := den.SetFields{
		"last_error":        errMsg,
		"error_class":       errorClass,
		"last_attempted_at": job.LastAttemptedAt,
		"locked_at":         nil,
		"worker_id":         "",
		"attempts":          job.Attempts,
	}

	if job.Attempts < job.MaxRetries {
		backoff := baseDelay * time.Duration(math.Pow(2, float64(job.Attempts-1)))
		fields["status"] = string(StatusFailed)
		fields["run_at"] = now.Add(backoff)
	} else {
		fields["status"] = string(StatusDead)
		fields["failed_at"] = &now
	}

	_, err := den.FindOneAndUpdate[Job](ctx, r.db, fields,
		where.Field("_id").Eq(job.ID),
		where.Field("status").Eq(string(StatusRunning)),
		where.Field("worker_id").Eq(job.WorkerID),
	)
	if errors.Is(err, den.ErrNotFound) {
		return ErrStaleJob
	}
	if err != nil {
		return fmt.Errorf("fail job %s: %w", job.ID, err)
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

// FindByID retrieves a job by ID.
func (r *Repository) FindByID(ctx context.Context, id string) (*Job, error) {
	return den.FindByID[Job](ctx, r.db, id)
}

// Delete deletes a job by ID (any status).
func (r *Repository) Delete(ctx context.Context, id string) error {
	job, err := den.FindByID[Job](ctx, r.db, id)
	if err != nil {
		return err
	}
	return den.Delete(ctx, r.db, job)
}

// GetResult returns the stored result for a job by ID. Returns ErrNotFound
// if the job does not exist.
func (r *Repository) GetResult(ctx context.Context, id string) (string, error) {
	job, err := den.FindByID[Job](ctx, r.db, id)
	if err != nil {
		return "", err
	}
	return job.Result, nil
}

// Retry resets a dead or failed job back to pending for re-processing.
func (r *Repository) Retry(ctx context.Context, id string) error {
	now := time.Now()
	_, err := den.FindOneAndUpdate[Job](ctx, r.db,
		den.SetFields{
			"status":            string(StatusPending),
			"attempts":          0,
			"last_error":        "",
			"error_class":       "",
			"result":            "",
			"last_attempted_at": nil,
			"failed_at":         nil,
			"locked_at":         nil,
			"run_at":            now,
			"worker_id":         "",
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
			"worker_id": "",
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
// given duration back to pending status. Each update is guarded by a
// status check to prevent resetting jobs that completed between the
// initial query and the update.
func (r *Repository) RescueStale(ctx context.Context, staleDuration time.Duration) (int64, error) {
	cutoffStr := time.Now().Add(-staleDuration).Format(time.RFC3339Nano)

	stale, err := den.NewQuery[Job](ctx, r.db,
		where.Field("status").Eq(string(StatusRunning)),
		where.Field("locked_at").Lt(cutoffStr),
	).All()
	if err != nil {
		return 0, fmt.Errorf("rescue stale jobs: %w", err)
	}

	now := time.Now()
	var count int64
	for _, job := range stale {
		_, err := den.FindOneAndUpdate[Job](ctx, r.db,
			den.SetFields{
				"status":    string(StatusPending),
				"locked_at": nil,
				"worker_id": "",
				"run_at":    now,
			},
			where.Field("_id").Eq(job.ID),
			where.Field("status").Eq(string(StatusRunning)),
		)
		if err != nil {
			if errors.Is(err, den.ErrNotFound) {
				continue // Job was completed/failed between query and update — skip.
			}
			return count, fmt.Errorf("rescue stale job %s: %w", job.ID, err)
		}
		count++
	}
	return count, nil
}
