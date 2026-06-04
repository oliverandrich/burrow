package server

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/oliverandrich/burrow/app"
	"github.com/stretchr/testify/assert"
)

// runForwarded sends req through forwardedHeadersMiddleware(cfg) and returns
// the (possibly mutated) request the inner handler saw.
func runForwarded(t *testing.T, cfg app.ForwardedConfig, req *http.Request) *http.Request {
	t.Helper()
	var got *http.Request
	h := forwardedHeadersMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.WriteHeader(http.StatusOK)
	}))
	h.ServeHTTP(httptest.NewRecorder(), req)
	return got
}

func proxiedReq(remoteAddr, proto string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/", nil)
	r.RemoteAddr = remoteAddr
	if proto != "" {
		r.Header.Set("X-Forwarded-Proto", proto)
	}
	return r
}

func TestForwarded_PublicPeerIgnored(t *testing.T) {
	// Headline spoofing defense: an untrusted public peer cannot forge the scheme.
	got := runForwarded(t, app.ForwardedConfig{Mode: "private"}, proxiedReq("203.0.113.5:1234", "https"))
	assert.Nil(t, got.TLS, "public peer's X-Forwarded-Proto must be ignored")
	assert.Empty(t, app.ForwardedProto(got.Context()))
}

func TestForwarded_CGNATPeerIgnored(t *testing.T) {
	// Shared-hosting (Uberspace 100.64/10) neighbours must not be trusted by default.
	got := runForwarded(t, app.ForwardedConfig{Mode: "private"}, proxiedReq("100.64.60.2:1234", "https"))
	assert.Nil(t, got.TLS)
}

func TestForwarded_DefaultPrivate_LoopbackTrusted(t *testing.T) {
	got := runForwarded(t, app.ForwardedConfig{Mode: "private"}, proxiedReq("127.0.0.1:5000", "https"))
	assert.NotNil(t, got.TLS, "loopback proxy is trusted under private mode")
	assert.Equal(t, "https", app.ForwardedProto(got.Context()))
	assert.True(t, app.RequestIsHTTPS(got))
}

func TestForwarded_DoesNotCorruptURL(t *testing.T) {
	// A real server request carries an origin-form URL (no scheme/host; the
	// host lives in r.Host). Setting r.URL.Scheme alone would make
	// r.URL.String() return "https:///auth/login" and pollute logs / absolute
	// URLs. The scheme signal must flow via the context flag + r.TLS sentinel
	// instead, leaving r.URL untouched.
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/login", nil)
	r.RemoteAddr = "127.0.0.1:5000"
	r.Header.Set("X-Forwarded-Proto", "https")

	got := runForwarded(t, app.ForwardedConfig{Mode: "private"}, r)

	assert.Equal(t, "/auth/login", got.URL.String(), "origin-form request URL must stay intact")
	assert.Empty(t, got.URL.Scheme)
	assert.True(t, app.RequestIsHTTPS(got), "scheme still signaled via context flag")
	assert.NotNil(t, got.TLS, "and via the r.TLS sentinel")
}

func TestForwarded_DefaultPrivate_RFC1918Trusted(t *testing.T) {
	got := runForwarded(t, app.ForwardedConfig{Mode: "private"}, proxiedReq("10.1.2.3:5000", "https"))
	assert.NotNil(t, got.TLS)
	assert.Equal(t, "https", app.ForwardedProto(got.Context()))
}

func TestForwarded_NoDowngrade(t *testing.T) {
	// A genuine TLS connection must not be downgraded by X-Forwarded-Proto: http.
	req := proxiedReq("127.0.0.1:5000", "http")
	req.TLS = &tls.ConnectionState{}
	got := runForwarded(t, app.ForwardedConfig{Mode: "private"}, req)
	assert.NotNil(t, got.TLS, "existing r.TLS must survive an http forwarded-proto")
}

func TestForwarded_LoopbackMode(t *testing.T) {
	cfg := app.ForwardedConfig{Mode: "loopback"}
	assert.Nil(t, runForwarded(t, cfg, proxiedReq("10.1.2.3:5000", "https")).TLS, "loopback mode rejects RFC1918")
	assert.NotNil(t, runForwarded(t, cfg, proxiedReq("127.0.0.1:5000", "https")).TLS, "loopback mode trusts loopback")
}

func TestForwarded_OffMode(t *testing.T) {
	got := runForwarded(t, app.ForwardedConfig{Mode: "off"}, proxiedReq("127.0.0.1:5000", "https"))
	assert.Nil(t, got.TLS, "off mode ignores forwarded headers from any peer")
}

func TestForwarded_TrustHostGate(t *testing.T) {
	mk := func() *http.Request {
		r := proxiedReq("127.0.0.1:5000", "https")
		r.Header.Set("X-Forwarded-Host", "public.example.com")
		return r
	}
	off := runForwarded(t, app.ForwardedConfig{Mode: "private"}, mk())
	assert.Equal(t, "example.test", off.Host, "X-Forwarded-Host ignored without trust-host")

	on := runForwarded(t, app.ForwardedConfig{Mode: "private", TrustHost: true}, mk())
	assert.Equal(t, "public.example.com", on.Host, "X-Forwarded-Host honored with trust-host")
}

func TestPeerInTrustedCIDRs(t *testing.T) {
	private := privatePrefixes
	tests := []struct {
		name       string
		remoteAddr string
		prefixes   []netip.Prefix
		want       bool
	}{
		{"loopback v4", "127.0.0.1:80", private, true},
		{"loopback v6", "[::1]:80", private, true},
		{"rfc1918 in", "10.1.2.3:80", private, true},
		{"rfc1918 192.168 in", "192.168.1.1:80", private, true},
		{"public out", "203.0.113.5:80", private, false},
		{"cgnat out", "100.64.60.2:80", private, false},
		{"ula v6 in", "[fd00::1]:80", private, true},
		{"v4-mapped-v6", "[::ffff:10.0.0.1]:80", private, true},
		{"missing port", "10.0.0.1", private, true},
		{"malformed", "not-an-addr", private, false},
		{"loopback-only set rejects rfc1918", "10.0.0.1:80", loopbackPrefixes, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, peerInTrustedCIDRs(tc.remoteAddr, tc.prefixes))
		})
	}
}
