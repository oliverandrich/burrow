package session

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/securecookie"
	"github.com/oliverandrich/burrow/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == "_session" {
			return c
		}
	}
	t.Fatal("no _session cookie in response")
	return nil
}

// TestSession_SecureCookieUnderForwardedHTTPS: boot config is HTTP (secure
// false), but a request proxied over HTTPS (forwarded-proto flag) must still
// get a Secure session cookie.
func TestSession_SecureCookieUnderForwardedHTTPS(t *testing.T) {
	r, sApp := routerWithSession(t)
	require.False(t, sApp.manager.secure, "boot default is insecure (no https base URL)")

	r.Get("/set", func(w http.ResponseWriter, req *http.Request) {
		assert.NoError(t, Set(w, req, "k", "v"))
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(app.WithForwardedProto(t.Context(), "https"), http.MethodGet, "/set", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.True(t, sessionCookie(t, rec).Secure, "forwarded-https request gets a Secure cookie")
}

func TestSession_PlainHTTPStaysInsecure(t *testing.T) {
	r, _ := routerWithSession(t)
	r.Get("/set", func(w http.ResponseWriter, req *http.Request) {
		assert.NoError(t, Set(w, req, "k", "v"))
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/set", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.False(t, sessionCookie(t, rec).Secure, "plain HTTP request keeps an insecure cookie")
}

func TestSession_ClearHonorsForwardedSecure(t *testing.T) {
	r, _ := routerWithSession(t)
	r.Get("/clear", func(w http.ResponseWriter, req *http.Request) {
		Clear(w, req)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(app.WithForwardedProto(t.Context(), "https"), http.MethodGet, "/clear", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	c := sessionCookie(t, rec)
	assert.True(t, c.Secure, "deletion cookie is Secure under forwarded-https")
	assert.Equal(t, -1, c.MaxAge, "Clear writes a deletion cookie")
}

// TestSession_HTTPSBootNotDowngraded: an https-configured app must keep Secure
// cookies even on a stray plain-HTTP request (upgrade-only — boot is the floor).
func TestSession_HTTPSBootNotDowngraded(t *testing.T) {
	sc := securecookie.New(make([]byte, 32), nil)
	sc.MaxAge(3600)
	a := &App{manager: &Manager{sc: sc, cookieName: "_session", maxAge: 3600, secure: true}}

	r := chi.NewRouter()
	r.Use(a.Middleware()[0])
	r.Get("/set", func(w http.ResponseWriter, req *http.Request) {
		assert.NoError(t, Set(w, req, "k", "v"))
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/set", nil) // no forwarded flag, r.TLS nil
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.True(t, sessionCookie(t, rec).Secure, "https boot default is never downgraded")
}
