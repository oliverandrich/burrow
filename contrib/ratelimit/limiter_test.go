package ratelimit

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLimiter_AllowWithinBurst(t *testing.T) {
	l := NewLimiter(10, 5, time.Minute)
	defer l.Stop()

	for i := range 5 {
		allowed, _ := l.Allow("client-a")
		assert.True(t, allowed, "request %d should be allowed within burst", i)
	}
}

func TestLimiter_DenyAfterBurst(t *testing.T) {
	l := NewLimiter(10, 3, time.Minute)
	defer l.Stop()

	for range 3 {
		allowed, _ := l.Allow("client-a")
		require.True(t, allowed)
	}

	allowed, retryAfter := l.Allow("client-a")
	assert.False(t, allowed, "should be denied after burst exhausted")
	assert.Greater(t, retryAfter, time.Duration(0), "retryAfter should be positive")
}

func TestLimiter_IndependentKeys(t *testing.T) {
	l := NewLimiter(10, 1, time.Minute)
	defer l.Stop()

	allowed, _ := l.Allow("client-a")
	assert.True(t, allowed)

	allowed, _ = l.Allow("client-b")
	assert.True(t, allowed, "different keys should have independent buckets")
}

func TestLimiter_RefillOverTime(t *testing.T) {
	// 100 req/s, burst 1 => refills 1 token in 10ms.
	l := NewLimiter(100, 1, time.Minute)
	defer l.Stop()

	allowed, _ := l.Allow("client-a")
	require.True(t, allowed)

	allowed, _ = l.Allow("client-a")
	require.False(t, allowed)

	time.Sleep(15 * time.Millisecond)

	allowed, _ = l.Allow("client-a")
	assert.True(t, allowed, "should be allowed after token refill")
}

func TestLimiter_RetryAfterDuration(t *testing.T) {
	// 1 req/s, burst 1.
	l := NewLimiter(1, 1, time.Minute)
	defer l.Stop()

	allowed, _ := l.Allow("client-a")
	require.True(t, allowed)

	allowed, retryAfter := l.Allow("client-a")
	require.False(t, allowed)
	// Should be roughly 1 second (give or take).
	assert.InDelta(t, time.Second.Seconds(), retryAfter.Seconds(), 0.1)
}

func TestLimiter_ConcurrentAccess(t *testing.T) {
	l := NewLimiter(1000, 100, time.Minute)
	defer l.Stop()

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			for range 20 {
				l.Allow("shared-key")
			}
		})
	}
	wg.Wait()
}

func TestLimiter_CleanupRemovesStaleEntries(t *testing.T) {
	// 1000 req/s, burst 1, cleanup every 10ms.
	l := NewLimiter(1000, 1, 10*time.Millisecond)
	defer l.Stop()

	l.Allow("stale-client")

	// Wait for cleanup to run.
	time.Sleep(50 * time.Millisecond)

	l.mu.Lock()
	_, exists := l.buckets["stale-client"]
	l.mu.Unlock()

	assert.False(t, exists, "stale entry should be cleaned up")
}

func TestLimiter_ActiveEntriesNotCleaned(t *testing.T) {
	l := NewLimiter(1000, 10, 10*time.Millisecond)
	defer l.Stop()

	// Keep using the key.
	for range 5 {
		l.Allow("active-client")
		time.Sleep(5 * time.Millisecond)
	}

	l.mu.Lock()
	_, exists := l.buckets["active-client"]
	l.mu.Unlock()

	assert.True(t, exists, "active entry should not be cleaned up")
}

func TestLimiter_StopIsIdempotent(t *testing.T) {
	l := NewLimiter(10, 5, time.Minute)
	l.Stop()
	l.Stop() // Should not panic.
}
