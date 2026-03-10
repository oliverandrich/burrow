package jobs

import (
	"time"

	"github.com/uptrace/bun"
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
type Job struct { //nolint:govet // fieldalignment: readability over optimization
	bun.BaseModel `bun:"table:_jobs,alias:j"`

	ID          int64      `bun:",pk,autoincrement" json:"id" verbose:"ID"`
	Type        string     `bun:",notnull" json:"type" verbose:"Type"`
	Payload     string     `bun:",notnull,default:'{}'" json:"payload"`
	Status      JobStatus  `bun:",notnull,default:'pending'" json:"status" verbose:"Status"`
	Attempts    int        `bun:",notnull,default:0" json:"attempts" verbose:"Attempts"`
	MaxRetries  int        `bun:",notnull,default:3" json:"max_retries"`
	RunAt       time.Time  `bun:",notnull,default:current_timestamp" json:"run_at"`
	LockedAt    *time.Time `bun:",nullzero" json:"locked_at,omitempty"`
	CompletedAt *time.Time `bun:",nullzero" json:"completed_at,omitempty"`
	FailedAt    *time.Time `bun:",nullzero" json:"failed_at,omitempty"`
	LastError   string     `bun:",nullzero" json:"last_error,omitempty"`
	CreatedAt   time.Time  `bun:",nullzero,notnull,default:current_timestamp" json:"created_at" verbose:"Created at"`
	UpdatedAt   *time.Time `bun:",nullzero" json:"updated_at,omitempty"`
}
