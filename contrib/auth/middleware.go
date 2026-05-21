package auth

import (
	"net/http"
	"net/url"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/session"
)

// requireAuth returns middleware that redirects to login if not authenticated.
func requireAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsAuthenticated(r.Context()) {
				if target := redirectTarget(r); target != "" {
					_ = session.Set(w, r, "redirect_after_login", target)
				}
				redirectToLogin(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// redirectTarget returns the URL to redirect back to after login.
// For GET requests it returns the request URI; for other methods it
// extracts the path from the Referer header (to avoid storing a
// POST-only URL that would 405 on GET redirect).
func redirectTarget(r *http.Request) string {
	if r.Method == http.MethodGet {
		return r.URL.RequestURI()
	}
	ref := r.Header.Get("Referer")
	if ref == "" {
		return ""
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	return parsed.RequestURI()
}

// roleChecker is the minimal interface implemented by every *User[P] —
// the middleware stays non-generic by reading the user via this interface
// rather than the parameterised User type.
type roleChecker interface {
	IsStaff() bool
	IsAdmin() bool
}

// requireRole returns middleware that loads the request's user from
// context and applies pass to decide access. Unauthenticated requests
// redirect to login; authenticated requests where pass returns false get a
// 403 error page.
func requireRole(pass func(roleChecker) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value(ctxKeyUser{}).(roleChecker)
			if !ok {
				if target := redirectTarget(r); target != "" {
					_ = session.Set(w, r, "redirect_after_login", target)
				}
				redirectToLogin(w, r)
				return
			}
			if !pass(user) {
				burrow.RenderError(w, r, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requireStaff() func(http.Handler) http.Handler {
	return requireRole(roleChecker.IsStaff)
}

func requireAdmin() func(http.Handler) http.Handler {
	return requireRole(roleChecker.IsAdmin)
}

// redirectToLogin sends the user to the login page. For HTMX requests it uses
// HX-Redirect to force a full page navigation instead of swapping into the
// current target (which would render the login page inside <main>).
func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	htmx.SmartRedirect(w, r, "/auth/login")
}

// RequireAuth returns middleware that redirects to login if not authenticated.
// The original request URL is stored in the session as "redirect_after_login"
// so the user can be redirected back after successful authentication.
//
// For GET requests, the full request URI is stored. For other methods (POST,
// PUT, DELETE, etc.), the Referer header is used instead — since the redirect
// back is always a GET, storing a POST-only URL would cause a 405.
func RequireAuth() func(http.Handler) http.Handler { return requireAuth() }

// RequireStaff returns middleware that enforces staff (or admin) access.
// Unauthenticated users are redirected to login (like [RequireAuth]).
// Authenticated non-staff users see a 403 error page.
func RequireStaff() func(http.Handler) http.Handler { return requireStaff() }

// RequireAdmin returns middleware that enforces admin access.
// Unauthenticated users are redirected to login (like [RequireAuth]).
// Authenticated non-admin users see a 403 error page.
func RequireAdmin() func(http.Handler) http.Handler { return requireAdmin() }

// Compile-time check: auth.App[EmptyProfile] implements burrow.AdminAuth.
// The check holds for any P because the methods don't depend on P.
var _ burrow.AdminAuth = (*App[EmptyProfile])(nil)

// RequireAuth satisfies the burrow.AdminAuth interface so the admin app
// can discover auth middleware from the registry without importing this package.
func (a *App[P]) RequireAuth() func(http.Handler) http.Handler { return requireAuth() }

// RequireStaff satisfies the burrow.AdminAuth interface.
func (a *App[P]) RequireStaff() func(http.Handler) http.Handler { return requireStaff() }

// RequireAdmin satisfies the burrow.AdminAuth interface.
func (a *App[P]) RequireAdmin() func(http.Handler) http.Handler { return requireAdmin() }

// SafeRedirectPath validates a redirect path, falling back to defaultPath.
func SafeRedirectPath(next, defaultPath string) string {
	if next == "" {
		return defaultPath
	}
	parsed, err := url.Parse(next)
	if err != nil || parsed.Host != "" || parsed.Scheme != "" {
		return defaultPath
	}
	return next
}
