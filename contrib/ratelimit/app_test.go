package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ burrow.App           = (*App)(nil)
	_ burrow.Configurable  = (*App)(nil)
	_ burrow.HasMiddleware = (*App)(nil)
	_ burrow.HasShutdown   = (*App)(nil)
)

func TestName(t *testing.T) {
	a := New()
	assert.Equal(t, "ratelimit", a.Name())
}

func newTestApp(t *testing.T, rps float64, burst int) *App {
	t.Helper()
	a := New()
	a.configure(rps, burst, false)
	t.Cleanup(func() { a.Shutdown(t.Context()) })
	return a
}

func TestMiddleware_AllowsWithinLimit(t *testing.T) {
	a := newTestApp(t, 100, 5)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := a.Middleware()[0](inner)

	for range 5 {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}
}

func TestMiddleware_Returns429WhenExceeded(t *testing.T) {
	a := newTestApp(t, 1, 1)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := a.Middleware()[0](inner)

	// First request: allowed.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// Second request: denied.
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestMiddleware_RetryAfterHeader(t *testing.T) {
	a := newTestApp(t, 1, 1)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := a.Middleware()[0](inner)

	// Exhaust the burst.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// This request should get Retry-After.
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	retryAfter := rr.Header().Get("Retry-After")
	require.NotEmpty(t, retryAfter)

	secs, err := strconv.Atoi(retryAfter)
	require.NoError(t, err)
	assert.Positive(t, secs)
}

func TestMiddleware_DifferentIPsIndependent(t *testing.T) {
	a := newTestApp(t, 1, 1)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := a.Middleware()[0](inner)

	// Client A.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Client B — should also be allowed.
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "5.6.7.8:5678"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMiddleware_TrustProxy(t *testing.T) {
	a := New(WithKeyFunc(defaultKeyFunc(true)))
	a.configure(1, 1, true)
	t.Cleanup(func() { a.Shutdown(t.Context()) })

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := a.Middleware()[0](inner)

	// First request from proxy with X-Forwarded-For.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// Second request from same real IP — should be denied.
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestMiddleware_CustomKeyFunc(t *testing.T) {
	a := New(WithKeyFunc(func(r *http.Request) string {
		return r.Header.Get("X-API-Key")
	}))
	a.configure(1, 1, false)
	t.Cleanup(func() { a.Shutdown(t.Context()) })

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := a.Middleware()[0](inner)

	// Same IP but different API keys — should both be allowed.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	req.Header.Set("X-API-Key", "key-a")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	req.Header.Set("X-API-Key", "key-b")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMiddleware_CustomOnLimited(t *testing.T) {
	customCalled := false
	a := New(WithOnLimited(func(w http.ResponseWriter, _ *http.Request) {
		customCalled = true
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	a.configure(1, 1, false)
	t.Cleanup(func() { a.Shutdown(t.Context()) })

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := a.Middleware()[0](inner)

	// Exhaust burst.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Trigger custom handler.
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.True(t, customCalled)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestMiddleware_RetryAfterContext(t *testing.T) {
	a := newTestApp(t, 1, 1)

	var gotRetryAfter bool
	a.onLimited = func(w http.ResponseWriter, r *http.Request) {
		gotRetryAfter = RetryAfter(r.Context()) > 0
		w.WriteHeader(http.StatusTooManyRequests)
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := a.Middleware()[0](inner)

	// Exhaust burst.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	// Trigger limited.
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.True(t, gotRetryAfter, "RetryAfter should be available in context")
}

func TestShutdown(t *testing.T) {
	a := newTestApp(t, 10, 5)
	err := a.Shutdown(t.Context())
	assert.NoError(t, err)
}
