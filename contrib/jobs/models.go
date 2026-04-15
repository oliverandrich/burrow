package jobs

import (
	"errors"
	"time"

	"github.com/oliverandrich/den/document"
)

// ErrStaleJob is returned when a Complete or Fail operation fails because
// the job is no longer owned by this worker (e.g., it was reclaimed).
var ErrStaleJob = errors.New("job is no longer owned by this worker")

// JobStatus represents the state of a job in the queue.
type JobStatus string

// Job status constants.
const (
	StatusPending   JobStatus = "pending"
	StatusRunning   JobStatus = "running"
	StatusCompleted JobStatus = "completed"
	StatusFailed    JobStatus = "failed"
	StatusDead      JobStatus = "dead"
)

// Job represents a background job in the queue.
type Job struct {
	document.Base
	RunAt           time.Time  `json:"run_at"                    den:"index,index_together:claim"`
	Type            string     `json:"type"                      den:"index"`
	Payload         string     `json:"payload"`
	Status          JobStatus  `json:"status"                    den:"index,index_together:claim,index_together:stale"`
	Result          string     `json:"result,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	ErrorClass      string     `json:"error_class,omitempty"`
	LockedAt        *time.Time `json:"locked_at,omitempty"       den:"index_together:stale"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	FailedAt        *time.Time `json:"failed_at,omitempty"`
	LastAttemptedAt *time.Time `json:"last_attempted_at,omitempty"`
	Attempts        int        `json:"attempts"`
	MaxRetries      int        `json:"max_retries"`
	WorkerID        string     `json:"worker_id"`
}
