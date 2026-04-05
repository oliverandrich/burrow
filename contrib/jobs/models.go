package jobs

import (
	"time"

	"github.com/oliverandrich/den/document"
)

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
	RunAt       time.Time  `json:"run_at"                  den:"index"`
	Type        string     `json:"type"                    den:"index"`
	Payload     string     `json:"payload"`
	Status      JobStatus  `json:"status"                  den:"index"`
	LastError   string     `json:"last_error,omitempty"`
	LockedAt    *time.Time `json:"locked_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	FailedAt    *time.Time `json:"failed_at,omitempty"`
	Attempts    int        `json:"attempts"`
	MaxRetries  int        `json:"max_retries"`
}
