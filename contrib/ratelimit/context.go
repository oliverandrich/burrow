// Package ratelimit provides per-client rate limiting as a burrow contrib app.
// It uses a token bucket algorithm (via golang.org/x/time/rate) with automatic
// cleanup of idle entries.
package ratelimit

import (
	"context"
	"time"
)

type ctxKeyRetryAfter struct{}

// WithRetryAfter stores the retry-after duration in the context.
func WithRetryAfter(ctx context.Context, d time.Duration) context.Context {
	return context.WithValue(ctx, ctxKeyRetryAfter{}, d)
}

// RetryAfter returns how long the client should wait before retrying.
// Returns zero if the request was not rate-limited.
func RetryAfter(ctx context.Context) time.Duration {
	if d, ok := ctx.Value(ctxKeyRetryAfter{}).(time.Duration); ok {
		return d
	}
	return 0
}
