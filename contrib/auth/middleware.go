package auth

import (
	"net/http"
	"net/url"

	"github.com/oliverandrich/burrow/contrib/session"
)

// RequireAuth returns middleware that redirects to login if not authenticated.
// The original request URL is stored in the session as "redirect_after_login"
// so the user can be redirected back after successful authentication.
//
// For GET requests, the full request URI is stored. For other methods (POST,
// PUT, DELETE, etc.), the Referer header is used instead — since the redirect
// back is always a GET, storing a POST-only URL would cause a 405.
func RequireAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsAuthenticated(r.Context()) {
				if target := redirectTarget(r); target != "" {
					_ = session.Set(w, r, "redirect_after_login", target)
				}
				http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
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

// RequireAdmin returns middleware that returns 403 if the user is not an admin.
func RequireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil || !user.IsAdmin() {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

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
