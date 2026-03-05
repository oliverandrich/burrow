package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Limiter manages per-key rate limiters with automatic cleanup of idle entries.
type Limiter struct {
	buckets         map[string]*entry
	done            chan struct{}
	mu              sync.Mutex
	stopOnce        sync.Once
	cleanupInterval time.Duration
	rps             rate.Limit
	burst           int
}

// NewLimiter creates a Limiter with the given rate (requests per second),
// burst size, and cleanup interval. The cleanup goroutine starts immediately.
func NewLimiter(rps float64, burst int, cleanupInterval time.Duration) *Limiter {
	l := &Limiter{
		buckets:         make(map[string]*entry),
		rps:             rate.Limit(rps),
		burst:           burst,
		cleanupInterval: cleanupInterval,
		done:            make(chan struct{}),
	}
	go l.cleanup()
	return l
}

// Allow reports whether a request for the given key is allowed.
// If denied, retryAfter indicates how long the caller should wait.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	e, ok := l.buckets[key]
	if !ok {
		e = &entry{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.buckets[key] = e
	}
	e.lastSeen = time.Now()
	lim := e.limiter
	l.mu.Unlock()

	r := lim.Reserve()
	if delay := r.Delay(); delay > 0 {
		r.Cancel()
		return false, delay
	}

	return true, 0
}

// Stop signals the cleanup goroutine to exit. Safe to call multiple times.
func (l *Limiter) Stop() {
	l.stopOnce.Do(func() {
		close(l.done)
	})
}

func (l *Limiter) cleanup() {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.done:
			return
		case <-ticker.C:
			l.sweep()
		}
	}
}

func (l *Limiter) sweep() {
	// Remove entries not seen for longer than 2× the time it takes
	// to fully refill the bucket.
	maxIdle := time.Duration(float64(l.burst) / float64(l.rps) * 2 * float64(time.Second))

	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	for key, e := range l.buckets {
		if now.Sub(e.lastSeen) > maxIdle {
			delete(l.buckets, key)
		}
	}
}
