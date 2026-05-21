package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow/burrowtest"
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
			u := &User[EmptyProfile]{Username: "test"}
			u.ID = "user-1"
			ctx := WithUser(r.Context(), u)
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

// runRoleGateCases exercises a role-based middleware against the three
// roles. The shared shape lets one helper cover both RequireAdmin and
// RequireStaff without duplicating the router wiring.
func runRoleGateCases(t *testing.T, mw func(http.Handler) http.Handler, cases []struct {
	name       string
	role       string
	wantStatus int
}) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					u := &User[EmptyProfile]{Username: "test", Role: tc.role}
					u.ID = "user-1"
					ctx := WithUser(r.Context(), u)
					ctx = burrowtest.ErrorExecContext(ctx)
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			r.Use(mw)
			r.Get("/admin", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code)
		})
	}
}

func TestRequireAdmin(t *testing.T) {
	runRoleGateCases(t, RequireAdmin(), []struct {
		name       string
		role       string
		wantStatus int
	}{
		{"forbids regular user", RoleUser, http.StatusForbidden},
		{"forbids staff", RoleStaff, http.StatusForbidden},
		{"allows admin", RoleAdmin, http.StatusOK},
	})
}

func TestRequireStaff(t *testing.T) {
	runRoleGateCases(t, RequireStaff(), []struct {
		name       string
		role       string
		wantStatus int
	}{
		{"forbids regular user", RoleUser, http.StatusForbidden},
		{"allows staff", RoleStaff, http.StatusOK},
		{"allows admin", RoleAdmin, http.StatusOK},
	})
}

func TestRequireAdminRedirectsUnauthenticated(t *testing.T) {
	assertRedirectsUnauthenticated(t, RequireAdmin())
}

func TestRequireStaffRedirectsUnauthenticated(t *testing.T) {
	assertRedirectsUnauthenticated(t, RequireStaff())
}

func assertRedirectsUnauthenticated(t *testing.T, mw func(http.Handler) http.Handler) {
	t.Helper()
	r := chi.NewRouter()
	r.Use(mw)
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
