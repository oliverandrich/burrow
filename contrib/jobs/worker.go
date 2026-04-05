package jobs

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/den/document"
)

// WorkerConfig holds configuration for the worker pool.
type WorkerConfig struct {
	NumWorkers     int
	PollInterval   time.Duration
	BatchSize      int
	StaleTimeout   time.Duration
	RetryBaseDelay time.Duration
}

// DefaultWorkerConfig returns sensible defaults.
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		NumWorkers:     2,
		PollInterval:   time.Second,
		BatchSize:      10,
		StaleTimeout:   10 * time.Minute,
		RetryBaseDelay: 30 * time.Second,
	}
}

// Worker manages a poller goroutine and a pool of worker goroutines.
type Worker struct { //nolint:govet // fieldalignment: readability over optimization
	id           string
	repo         *Repository
	handlers     map[string]burrow.JobHandlerFunc
	config       WorkerConfig
	templateExec burrow.TemplateExecutor
	jobs         chan *Job
	wg           sync.WaitGroup
	done         chan struct{}
}

// NewWorker creates a new Worker. The exec parameter may be nil when
// template rendering is not needed (e.g., in tests).
func NewWorker(repo *Repository, handlers map[string]burrow.JobHandlerFunc, config WorkerConfig, exec burrow.TemplateExecutor) *Worker {
	return &Worker{
		id:           document.NewID(),
		repo:         repo,
		handlers:     handlers,
		config:       config,
		templateExec: exec,
		jobs:         make(chan *Job, config.BatchSize),
		done:         make(chan struct{}),
	}
}

// Start runs the poller and workers. It blocks until ctx is cancelled
// and all in-flight jobs have finished.
func (w *Worker) Start(ctx context.Context) {
	// Start worker goroutines.
	for range w.config.NumWorkers {
		w.wg.Go(w.work)
	}

	// Run the poller in this goroutine.
	w.poll(ctx)

	// Poller stopped — close the channel so workers drain and exit.
	close(w.jobs)
	w.wg.Wait()
	close(w.done)
}

// Done returns a channel that is closed when all workers have stopped.
func (w *Worker) Done() <-chan struct{} {
	return w.done
}

func (w *Worker) poll(ctx context.Context) {
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	maintenanceTicker := time.NewTicker(5 * time.Minute)
	defer maintenanceTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.claimAndDispatch(ctx)
		case <-maintenanceTicker.C:
			w.maintenance(ctx)
		}
	}
}

func (w *Worker) claimAndDispatch(ctx context.Context) {
	claimed, err := w.repo.Claim(ctx, w.id, w.config.BatchSize)
	if err != nil {
		slog.Error("jobs: claim failed", "error", err)
		return
	}
	for _, j := range claimed {
		select {
		case w.jobs <- j:
		case <-ctx.Done():
			return
		}
	}
}

func (w *Worker) maintenance(ctx context.Context) {
	if n, err := w.repo.RescueStale(ctx, w.config.StaleTimeout); err != nil {
		slog.Error("jobs: rescue stale failed", "error", err)
	} else if n > 0 {
		slog.Info("jobs: rescued stale jobs", "count", n)
	}

	if n, err := w.repo.DeleteCompleted(ctx, 24*time.Hour); err != nil {
		slog.Error("jobs: delete completed failed", "error", err)
	} else if n > 0 {
		slog.Info("jobs: deleted old completed jobs", "count", n)
	}
}

func (w *Worker) work() {
	for job := range w.jobs {
		w.processJob(job)
	}
}

func (w *Worker) processJob(job *Job) {
	// Increment attempts and stamp ownership — persisted atomically via Complete or Fail.
	job.Attempts++
	job.WorkerID = w.id

	handler, ok := w.handlers[job.Type]
	if !ok {
		slog.Error("jobs: unknown job type", "type", job.Type, "id", job.ID)
		// Force dead immediately — unknown types should never retry.
		job.Attempts = job.MaxRetries
		if err := w.repo.Fail(context.Background(), job, "unknown job type: "+job.Type, w.config.RetryBaseDelay); err != nil {
			if errors.Is(err, ErrStaleJob) {
				slog.Warn("jobs: job was reclaimed by another worker", "id", job.ID)
				return
			}
			slog.Error("jobs: fail unknown job", "error", err, "id", job.ID)
		}
		return
	}

	// Handlers get a background context so they can finish even after
	// the poller context is cancelled. If a template executor is available,
	// inject it so handlers can use RenderFragment.
	ctx := context.Background()
	if w.templateExec != nil {
		ctx = burrow.WithTemplateExecutor(ctx, w.templateExec)
	}
	err := handler(ctx, []byte(job.Payload))
	if err != nil {
		slog.Error("jobs: handler failed", "type", job.Type, "id", job.ID, "error", err, "attempt", job.Attempts)
		if failErr := w.repo.Fail(context.Background(), job, err.Error(), w.config.RetryBaseDelay); failErr != nil {
			if errors.Is(failErr, ErrStaleJob) {
				slog.Warn("jobs: job was reclaimed by another worker", "id", job.ID)
				return
			}
			slog.Error("jobs: record failure", "error", failErr, "id", job.ID)
		}
		return
	}

	if err := w.repo.Complete(context.Background(), job); err != nil {
		if errors.Is(err, ErrStaleJob) {
			slog.Warn("jobs: job was reclaimed by another worker", "id", job.ID)
			return
		}
		slog.Error("jobs: complete failed", "error", err, "id", job.ID)
	}
}
