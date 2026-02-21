package auth

import (
	"net/http"
	"net/url"

	"codeberg.org/oliverandrich/burrow/contrib/session"
)

// RequireAuth returns middleware that redirects to login if not authenticated.
// The original request URL is stored in the session as "redirect_after_login"
// so the user can be redirected back after successful authentication.
func RequireAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsAuthenticated(r) {
				target := r.URL.RequestURI()
				_ = session.Set(w, r, "redirect_after_login", target)
				http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin returns middleware that returns 403 if the user is not an admin.
func RequireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUser(r)
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
