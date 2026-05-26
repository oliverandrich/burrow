package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/oliverandrich/burrow/app"
	"github.com/stretchr/testify/assert"
)

// runWithClientIPMiddleware exercises the dispatch helper end-to-end:
// wraps a handler with clientIPMiddleware(cfg), fires req, returns the
// client IP chi stored in the context (via chimw.GetClientIP). The handler
// also captures whatever was set so the test can assert on it.
func runWithClientIPMiddleware(t *testing.T, cfg app.ClientIPConfig, req *http.Request) string {
	t.Helper()
	var got string
	h := clientIPMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = chimw.GetClientIP(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	h.ServeHTTP(httptest.NewRecorder(), req)
	return got
}

func TestClientIPMiddleware_RemoteAddr(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:54321"

	got := runWithClientIPMiddleware(t, app.ClientIPConfig{Mode: "remote-addr"}, req)
	assert.Equal(t, "203.0.113.5", got)
}

func TestClientIPMiddleware_RemoteAddr_DefaultModeEmpty(t *testing.T) {
	// Mode == "" should behave the same as remote-addr (default path).
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:54321"

	got := runWithClientIPMiddleware(t, app.ClientIPConfig{}, req)
	assert.Equal(t, "203.0.113.5", got)
}

func TestClientIPMiddleware_Header(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:443" // proxy
	req.Header.Set("X-Real-IP", "198.51.100.7")

	got := runWithClientIPMiddleware(t, app.ClientIPConfig{
		Mode:   "header",
		Header: "X-Real-IP",
	}, req)
	assert.Equal(t, "198.51.100.7", got, "header mode should ignore RemoteAddr and trust the configured header")
}

func TestClientIPMiddleware_Header_Cloudflare(t *testing.T) {
	// Same mechanism, different header name — confirms the header is wired
	// from config rather than hardcoded.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:443"
	req.Header.Set("CF-Connecting-IP", "198.51.100.8")

	got := runWithClientIPMiddleware(t, app.ClientIPConfig{
		Mode:   "header",
		Header: "CF-Connecting-IP",
	}, req)
	assert.Equal(t, "198.51.100.8", got)
}

func TestClientIPMiddleware_XFFTrustedProxies(t *testing.T) {
	// Topology: Client (203.0.113.5) → TrustedProxy1 → TrustedProxy2 → us.
	// As-received XFF is "203.0.113.5, 10.0.0.1" (TrustedProxy2 appended
	// TrustedProxy1's IP). With numTrusted=2 chi returns
	// xff[len-2] = xff[0] = the client. A spoofed entry the client prepends
	// (simulated below by adding an evil first entry) sits OUTSIDE the
	// trusted zone and is correctly ignored.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:443"
	req.Header.Set("X-Forwarded-For", "192.0.2.99, 203.0.113.5, 10.0.0.1")

	got := runWithClientIPMiddleware(t, app.ClientIPConfig{
		Mode:           "xff-trusted-proxies",
		TrustedProxies: 2,
	}, req)
	assert.Equal(t, "203.0.113.5", got,
		"with 2 trusted proxies and a spoofed leading entry, chi must return position len(xff)-numTrusted")
}

func TestClientIPMiddleware_XFFTrustedCIDRs(t *testing.T) {
	// Trusted proxy CIDR covers 198.51.100.0/24; the first non-trusted IP
	// walking right-to-left is the client.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.50:443"
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 198.51.100.51")

	got := runWithClientIPMiddleware(t, app.ClientIPConfig{
		Mode:         "xff-trusted-cidrs",
		TrustedCIDRs: []string{"198.51.100.0/24"},
	}, req)
	assert.Equal(t, "203.0.113.5", got)
}

func TestClientIPMiddleware_UnknownMode_FallsBackToRemoteAddr(t *testing.T) {
	// Validation should prevent this in practice, but the dispatcher must
	// not panic — it falls back to the safest mode.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:54321"

	got := runWithClientIPMiddleware(t, app.ClientIPConfig{Mode: "bogus"}, req)
	assert.Equal(t, "203.0.113.5", got)
}
