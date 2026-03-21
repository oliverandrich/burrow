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
// If trustProxy is true, it uses the X-Real-IP header set by the reverse proxy.
// X-Forwarded-For is intentionally not used because it can contain multiple
// comma-separated values that are trivially spoofed to bypass rate limiting.
//
// WARNING: trustProxy must only be enabled when the application runs behind a
// reverse proxy (nginx, Caddy, etc.) that sets the X-Real-IP header and strips
// any client-supplied value. Without a proxy, clients can spoof X-Real-IP to
// bypass rate limiting entirely.
func defaultKeyFunc(trustProxy bool) func(*http.Request) string {
	return func(r *http.Request) string {
		if trustProxy {
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
