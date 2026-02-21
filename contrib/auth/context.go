// Package auth provides authentication as a burrow contrib app.
//
// It implements WebAuthn (passkeys), recovery codes, email verification,
// and invite-only registration. Context helpers provide access to the
// authenticated user from any handler.
package auth

import (
	"context"
	"net/http"
)

// ctxKeyUser is the context key for the authenticated user.
type ctxKeyUser struct{}

// GetUser retrieves the authenticated user from the request context.
func GetUser(r *http.Request) *User {
	if user, ok := r.Context().Value(ctxKeyUser{}).(*User); ok {
		return user
	}
	return nil
}

// IsAuthenticated returns true if a user is logged in.
func IsAuthenticated(r *http.Request) bool {
	return GetUser(r) != nil
}

// WithUser returns a new context with the user set.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, ctxKeyUser{}, user)
}
