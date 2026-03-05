package ratelimit

import (
	"fmt"
	"math"
	"net"
	"net/http"
)

func (a *App) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := a.keyFunc(r)

		allowed, retryAfter := a.limiter.Allow(key)
		if !allowed {
			ctx := WithRetryAfter(r.Context(), retryAfter)
			r = r.WithContext(ctx)

			secs := int(math.Ceil(retryAfter.Seconds()))
			w.Header().Set("Retry-After", fmt.Sprintf("%d", secs))

			a.onLimited(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// defaultKeyFunc extracts the client IP from the request.
// If trustProxy is true, it checks X-Forwarded-For and X-Real-IP first.
func defaultKeyFunc(trustProxy bool) func(*http.Request) string {
	return func(r *http.Request) string {
		if trustProxy {
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				return xff
			}
			if xri := r.Header.Get("X-Real-IP"); xri != "" {
				return xri
			}
		}

		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr
		}
		return host
	}
}

// defaultOnLimited sends a plain text 429 response.
func defaultOnLimited(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
}
