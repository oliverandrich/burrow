package jobs

import (
	"context"
	"time"
)

// statusCount pairs a job status with its current count for the status card.
type statusCount struct {
	Status JobStatus
	Count  int
}

// workerStatusView is the worker-pool health summary rendered above the admin
// job list: pool meta from the running worker plus per-status job counts.
type workerStatusView struct {
	LastPollAt time.Time
	Counts     []statusCount
	Workers    int
	InFlight   int
	Started    bool // a worker has been created (false before Start / in tests)
	Running    bool // the worker pool is live (false once drained)
	Stale      bool // the poller has not ticked within the readiness threshold
}

// workerStatus assembles the worker-status panel data: per-status job counts
// (always all five statuses) plus live pool metrics when a worker exists.
func (a *App) workerStatus(ctx context.Context) (workerStatusView, error) {
	counts, err := a.repo.CountByStatus(ctx)
	if err != nil {
		return workerStatusView{}, err
	}

	view := workerStatusView{
		Counts: []statusCount{
			{StatusPending, counts[StatusPending]},
			{StatusRunning, counts[StatusRunning]},
			{StatusFailed, counts[StatusFailed]},
			{StatusDead, counts[StatusDead]},
			{StatusCompleted, counts[StatusCompleted]},
		},
	}

	if a.worker != nil {
		s := a.worker.Stats()
		view.Started = true
		view.Running = s.Running
		view.Workers = s.NumWorkers
		view.InFlight = s.InFlight
		view.LastPollAt = s.LastPollAt
		view.Stale = pollStale(s.LastPollAt, time.Now(), readinessThreshold(a.workerCfg.PollInterval))
	}

	return view, nil
}
