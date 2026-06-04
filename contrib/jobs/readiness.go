package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	// readinessPollMultiplier sets how many poll intervals of silence are
	// tolerated before the poller is considered stalled.
	readinessPollMultiplier = 10
	// readinessMinThreshold floors the staleness window so a sub-second poll
	// interval doesn't produce a hair-trigger readiness check.
	readinessMinThreshold = 30 * time.Second
)

// readinessThreshold returns how long the poller may be silent before it is
// considered stalled: readinessPollMultiplier × the poll interval, floored at
// readinessMinThreshold.
func readinessThreshold(pollInterval time.Duration) time.Duration {
	t := readinessPollMultiplier * pollInterval
	if t < readinessMinThreshold {
		return readinessMinThreshold
	}
	return t
}

// pollStale reports whether the last poll is older than threshold relative to now.
func pollStale(lastPoll, now time.Time, threshold time.Duration) bool {
	return now.Sub(lastPoll) > threshold
}

// ReadinessCheck reports the jobs worker as unready when the poller has not
// ticked within the staleness threshold, so a stalled or dead worker turns
// /healthz/ready into 503. It implements burrow.ReadinessChecker.
//
// This is a liveness check on the poll loop, not a correctness check on the
// work: a poller that ticks but whose Claim fails every cycle (e.g. a lost DB
// connection) keeps lastPollAt fresh and stays ready. Those claim failures are
// logged in claimAndDispatch; alert on that log separately if you need it.
func (a *App) ReadinessCheck(_ context.Context) error {
	if a.worker == nil {
		return errors.New("jobs worker not started")
	}
	stats := a.worker.Stats()
	threshold := readinessThreshold(a.workerCfg.PollInterval)
	if pollStale(stats.LastPollAt, time.Now(), threshold) {
		return fmt.Errorf("jobs poller stalled: last poll %s ago (threshold %s)",
			time.Since(stats.LastPollAt).Round(time.Second), threshold)
	}
	return nil
}
