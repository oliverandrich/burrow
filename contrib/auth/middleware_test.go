package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/stretchr/testify/assert"
)

func TestRequireAuthRedirects(t *testing.T) {
	r := chi.NewRouter()
	r.Use(RequireAuth())
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/protected?foo=bar", nil)
	req = session.Inject(req, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/auth/login", rec.Header().Get("Location"))
}

func TestRequireAuthStoresRedirectInSession(t *testing.T) {
	var capturedValues map[string]any
	r := chi.NewRouter()
	r.Use(RequireAuth())
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/protected?foo=bar", nil)
	req = session.Inject(req, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Verify the redirect target is stored in session by reading it back.
	capturedValues = session.GetValues(req)
	assert.Equal(t, "/protected?foo=bar", capturedValues["redirect_after_login"])
}

func TestRequireAuthStoresRefererForPOST(t *testing.T) {
	r := chi.NewRouter()
	r.Use(RequireAuth())
	r.Post("/polls/1/vote", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/polls/1/vote", nil)
	req.Header.Set("Referer", "http://localhost:8080/polls/1")
	req = session.Inject(req, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	capturedValues := session.GetValues(req)
	assert.Equal(t, "/polls/1", capturedValues["redirect_after_login"])
}

func TestRequireAuthSkipsRedirectForPOSTWithoutReferer(t *testing.T) {
	r := chi.NewRouter()
	r.Use(RequireAuth())
	r.Post("/polls/1/vote", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/polls/1/vote", nil)
	req = session.Inject(req, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	capturedValues := session.GetValues(req)
	assert.Nil(t, capturedValues["redirect_after_login"])
}

func TestRequireAuthAllowsAuthenticated(t *testing.T) {
	r := chi.NewRouter()
	// Inject user into context before RequireAuth.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithUser(r.Context(), &User{ID: 1})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Use(RequireAuth())
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireAdmin(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		wantStatus int
	}{
		{"forbids non-admin", RoleUser, http.StatusForbidden},
		{"allows admin", RoleAdmin, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx := WithUser(r.Context(), &User{ID: 1, Role: tt.role})
					ctx = burrow.TestErrorExecContext(ctx)
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			r.Use(RequireAdmin())
			r.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestRequireAdminRedirectsUnauthenticated(t *testing.T) {
	r := chi.NewRouter()
	r.Use(RequireAdmin())
	r.Get("/admin", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/auth/login", rec.Header().Get("Location"))
}

func TestSafeRedirectPath(t *testing.T) {
	tests := []struct {
		next     string
		expected string
	}{
		{"/dashboard", "/dashboard"},
		{"/settings?tab=profile", "/settings?tab=profile"},
		{"", "/default"},
		{"https://evil.com/steal", "/default"},
		{"//evil.com", "/default"},
	}

	for _, tt := range tests {
		t.Run(tt.next, func(t *testing.T) {
			assert.Equal(t, tt.expected, SafeRedirectPath(tt.next, "/default"))
		})
	}
}
