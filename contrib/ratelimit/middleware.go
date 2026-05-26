package ratelimit

import (
	"fmt"
	"math"
	"net"
	"net/http"

	"github.com/oliverandrich/burrow"
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

// defaultKeyFunc derives the rate-limit key from the framework-wide client
// IP set by burrow's server-level ClientIP middleware (configured via
// --client-ip-mode; see docs/guide/client-ip.md). Falls back to the host
// portion of RemoteAddr when the middleware did not run — e.g. in unit tests
// that exercise this contrib in isolation.
func defaultKeyFunc(r *http.Request) string {
	if ip := burrow.ClientIP(r.Context()); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// defaultOnLimited sends a plain text 429 response.
func defaultOnLimited(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
}
