package auth

import (
	"net/http"
	"strings"

	"github.com/oliverandrich/burrow"
)

// RequireAPIKey returns middleware that authenticates a request via an API
// key presented as an `Authorization: Bearer <token>` header. It is the
// non-browser counterpart to session auth: a valid key loads its owning
// user into the request context — both [WithUser] and a [burrow.AuthChecker],
// exactly as the session middleware does — so [CurrentUser], the core
// navLinks function, and [RequireStaff]/[RequireAdmin] all behave
// identically regardless of how the request authenticated.
//
// Unlike the session middleware (which lets unauthenticated requests pass
// through to be redirected to login), this is a hard gate: a missing,
// malformed, unknown, expired, or inactive-user key is rejected with 401.
// The error uses [burrow.RenderError], so API clients sending
// `Accept: application/json` receive a JSON body.
//
// The key inherits its user's role; per-role gating is achieved by chaining
// the existing role middleware after it, e.g.
//
//	r.Use(authApp.RequireAPIKey())
//	r.Use(auth.RequireStaff())
func (a *App[P]) RequireAPIKey() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				unauthorized(w, r)
				return
			}

			key, err := a.repo.GetAPIKeyByHash(r.Context(), HashToken(token))
			if err != nil || key.IsExpired() {
				unauthorized(w, r)
				return
			}

			user, err := a.repo.GetUserByID(r.Context(), key.UserID)
			if err != nil || !user.IsActive {
				unauthorized(w, r)
				return
			}

			next.ServeHTTP(w, r.WithContext(setAuthContext(r.Context(), user)))
		})
	}
}

// bearerToken extracts the token from an `Authorization: Bearer <token>`
// header, or returns "" when the header is absent or not a bearer scheme.
func bearerToken(r *http.Request) string {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// unauthorized rejects a request with 401 and a Bearer challenge.
func unauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	burrow.RenderError(w, r, http.StatusUnauthorized, "unauthorized")
}
