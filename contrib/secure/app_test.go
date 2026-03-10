package secure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// Compile-time interface assertions.
var (
	_ burrow.App           = (*App)(nil)
	_ burrow.Configurable  = (*App)(nil)
	_ burrow.HasMiddleware = (*App)(nil)
)

func TestName(t *testing.T) {
	a := New()
	assert.Equal(t, "secure", a.Name())
}

// newTestApp creates a configured App for testing.
func newTestApp(t *testing.T, baseURL string, opts ...Option) *App {
	t.Helper()
	a := New(opts...)
	err := a.Register(&burrow.AppConfig{
		Config: &burrow.Config{
			Server: burrow.ServerConfig{BaseURL: baseURL},
		},
	})
	require.NoError(t, err)

	isHTTPS := len(baseURL) >= 8 && baseURL[:8] == "https://"
	a.configure(isHTTPS)
	return a
}

// serveRequest runs a GET request to "/" through the middleware and returns the response.
func serveRequest(t *testing.T, a *App) *httptest.ResponseRecorder {
	t.Helper()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mws := a.Middleware()
	require.Len(t, mws, 1)
	handler := mws[0](inner)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestDefaultHeaders(t *testing.T) {
	a := newTestApp(t, "http://localhost:8080")
	rr := serveRequest(t, a)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", rr.Header().Get("Referrer-Policy"))
}

func TestHSTSEnabledForHTTPS(t *testing.T) {
	a := newTestApp(t, "https://example.com",
		WithSSLProxyHeaders(map[string]string{"X-Forwarded-Proto": "https"}),
	)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.Middleware()[0](inner)

	// Simulate HTTPS via proxy header so unrolled/secure emits the STS header.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	sts := rr.Header().Get("Strict-Transport-Security")
	assert.Contains(t, sts, "max-age=63072000")
	assert.Contains(t, sts, "includeSubDomains")
	assert.Contains(t, sts, "preload")
}

func TestHSTSDisabledForHTTP(t *testing.T) {
	a := newTestApp(t, "http://localhost:8080")
	rr := serveRequest(t, a)

	assert.Empty(t, rr.Header().Get("Strict-Transport-Security"))
}

func TestCSPHeader(t *testing.T) {
	a := newTestApp(t, "http://localhost:8080",
		WithContentSecurityPolicy("default-src 'self'"),
	)
	rr := serveRequest(t, a)

	assert.Equal(t, "default-src 'self'", rr.Header().Get("Content-Security-Policy"))
}

func TestCSPNotSetByDefault(t *testing.T) {
	a := newTestApp(t, "http://localhost:8080")
	rr := serveRequest(t, a)

	assert.Empty(t, rr.Header().Get("Content-Security-Policy"))
}

func TestPermissionsPolicyHeader(t *testing.T) {
	a := newTestApp(t, "http://localhost:8080",
		WithPermissionsPolicy("camera=(), microphone=()"),
	)
	rr := serveRequest(t, a)

	assert.Equal(t, "camera=(), microphone=()", rr.Header().Get("Permissions-Policy"))
}

func TestCOOPHeader(t *testing.T) {
	a := newTestApp(t, "http://localhost:8080",
		WithCrossOriginOpenerPolicy("same-origin"),
	)
	rr := serveRequest(t, a)

	assert.Equal(t, "same-origin", rr.Header().Get("Cross-Origin-Opener-Policy"))
}

func TestSSLRedirect(t *testing.T) {
	a := newTestApp(t, "https://example.com",
		WithSSLRedirect(true),
	)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.Middleware()[0](inner)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com/path", nil)
	req.Host = "example.com"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMovedPermanently, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "https://")
}

func TestAllowedHosts(t *testing.T) {
	a := newTestApp(t, "https://example.com",
		WithAllowedHosts("example.com"),
	)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.Middleware()[0](inner)

	// Allowed host.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Host = "example.com"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Disallowed host.
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Host = "evil.com"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestDevelopmentModeDisablesSTS(t *testing.T) {
	a := newTestApp(t, "https://example.com",
		WithDevelopment(true),
	)
	rr := serveRequest(t, a)

	assert.Empty(t, rr.Header().Get("Strict-Transport-Security"))
}

func TestDevelopmentModeDisablesHostCheck(t *testing.T) {
	a := newTestApp(t, "https://example.com",
		WithDevelopment(true),
		WithAllowedHosts("example.com"),
	)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.Middleware()[0](inner)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Host = "evil.com"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestConstructorOptions(t *testing.T) {
	a := newTestApp(t, "http://localhost:8080",
		WithContentSecurityPolicy("default-src 'self'"),
		WithPermissionsPolicy("camera=()"),
		WithCrossOriginOpenerPolicy("same-origin"),
	)
	rr := serveRequest(t, a)

	assert.Equal(t, "default-src 'self'", rr.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "camera=()", rr.Header().Get("Permissions-Policy"))
	assert.Equal(t, "same-origin", rr.Header().Get("Cross-Origin-Opener-Policy"))
}

func TestConstructorOptionsPrecedence(t *testing.T) {
	// Constructor sets CSP — should not be overridden by flags.
	csp := "default-src 'self'"
	a := New(WithContentSecurityPolicy(csp))
	err := a.Register(&burrow.AppConfig{
		Config: &burrow.Config{
			Server: burrow.ServerConfig{BaseURL: "http://localhost:8080"},
		},
	})
	require.NoError(t, err)

	// Simulate what Configure() does: if constructor already set a value,
	// the flag value is ignored.
	assert.NotNil(t, a.csp, "constructor should have set csp")
	assert.Equal(t, csp, *a.csp)
}

func TestSSLProxyHeaders(t *testing.T) {
	a := newTestApp(t, "https://example.com",
		WithSSLRedirect(true),
		WithSSLProxyHeaders(map[string]string{"X-Forwarded-Proto": "https"}),
	)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.Middleware()[0](inner)

	// Request with proxy header indicating HTTPS — should not redirect.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com/path", nil)
	req.Host = "example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHTTPAutoDetectsDevelopmentMode(t *testing.T) {
	// HTTP base URL without explicit development flag → IsDevelopment should be true.
	a := newTestApp(t, "http://localhost:8080")
	rr := serveRequest(t, a)

	// Development mode still sets default headers.
	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
	// But no STS.
	assert.Empty(t, rr.Header().Get("Strict-Transport-Security"))
}

func TestFlags(t *testing.T) {
	a := New()
	flags := a.Flags(nil)

	names := make(map[string]bool)
	for _, f := range flags {
		names[f.Names()[0]] = true
	}

	assert.Len(t, flags, 6)
	assert.True(t, names["secure-csp"])
	assert.True(t, names["secure-permissions-policy"])
	assert.True(t, names["secure-coop"])
	assert.True(t, names["secure-allowed-hosts"])
	assert.True(t, names["secure-ssl-redirect"])
	assert.True(t, names["secure-development"])
}

// configuredAppViaCLI creates a secure App configured through CLI flags.
func configuredAppViaCLI(t *testing.T, baseURL string, args []string, opts ...Option) *App {
	t.Helper()
	a := New(opts...)
	err := a.Register(&burrow.AppConfig{
		Config: &burrow.Config{
			Server: burrow.ServerConfig{BaseURL: baseURL},
		},
	})
	require.NoError(t, err)

	cmd := &cli.Command{
		Name:  "test",
		Flags: a.Flags(nil),
		Action: func(_ context.Context, cmd *cli.Command) error {
			return a.Configure(cmd)
		},
	}
	fullArgs := append([]string{"test"}, args...)
	err = cmd.Run(t.Context(), fullArgs)
	require.NoError(t, err)
	return a
}

func TestConfigureCSPFromFlag(t *testing.T) {
	a := configuredAppViaCLI(t, "http://localhost:8080", []string{
		"--secure-csp", "default-src 'none'",
	})
	rr := serveRequest(t, a)
	assert.Equal(t, "default-src 'none'", rr.Header().Get("Content-Security-Policy"))
}

func TestConfigurePermissionsPolicyFromFlag(t *testing.T) {
	a := configuredAppViaCLI(t, "http://localhost:8080", []string{
		"--secure-permissions-policy", "geolocation=()",
	})
	rr := serveRequest(t, a)
	assert.Equal(t, "geolocation=()", rr.Header().Get("Permissions-Policy"))
}

func TestConfigureCOOPFromFlag(t *testing.T) {
	a := configuredAppViaCLI(t, "http://localhost:8080", []string{
		"--secure-coop", "same-origin-allow-popups",
	})
	rr := serveRequest(t, a)
	assert.Equal(t, "same-origin-allow-popups", rr.Header().Get("Cross-Origin-Opener-Policy"))
}

func TestConfigureAllowedHostsFromFlag(t *testing.T) {
	a := configuredAppViaCLI(t, "https://example.com", []string{
		"--secure-allowed-hosts", "example.com, api.example.com",
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.Middleware()[0](inner)

	// Allowed host.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Host = "api.example.com"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Disallowed host.
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Host = "evil.com"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestConfigureSSLRedirectFromFlag(t *testing.T) {
	a := configuredAppViaCLI(t, "https://example.com", []string{
		"--secure-ssl-redirect",
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.Middleware()[0](inner)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com/path", nil)
	req.Host = "example.com"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMovedPermanently, rr.Code)
}

func TestConfigureDevelopmentFromFlag(t *testing.T) {
	a := configuredAppViaCLI(t, "https://example.com", []string{
		"--secure-development",
		"--secure-allowed-hosts", "example.com",
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.Middleware()[0](inner)

	// Development mode should allow any host even with allowed hosts set.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Host = "evil.com"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestConfigureConstructorOptionsTakePrecedence(t *testing.T) {
	// Constructor sets CSP; CLI flag should be ignored.
	a := configuredAppViaCLI(t, "http://localhost:8080", []string{
		"--secure-csp", "default-src 'none'",
	}, WithContentSecurityPolicy("script-src 'self'"))

	rr := serveRequest(t, a)
	assert.Equal(t, "script-src 'self'", rr.Header().Get("Content-Security-Policy"))
}

func TestConfigureNoFlags(t *testing.T) {
	// Configure with no flags set — should work and produce default headers.
	a := configuredAppViaCLI(t, "http://localhost:8080", nil)
	rr := serveRequest(t, a)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
	assert.Empty(t, rr.Header().Get("Content-Security-Policy"))
}
