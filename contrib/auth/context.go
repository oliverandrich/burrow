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

// UserFromContext retrieves the authenticated user from a context.
// This is useful in templ templates where only context.Context is available.
func UserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value(ctxKeyUser{}).(*User); ok {
		return user
	}
	return nil
}

// WithUser returns a new context with the user set.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, ctxKeyUser{}, user)
}

// Admin edit flags — set by UserDetail handler, read by templates.

type ctxKeyAdminEditFlags struct{}

type adminEditFlags struct {
	isSelf      bool
	isLastAdmin bool
}

// withAdminEditFlags returns a context with admin edit UI flags set.
func withAdminEditFlags(ctx context.Context, isSelf, isLastAdmin bool) context.Context {
	return context.WithValue(ctx, ctxKeyAdminEditFlags{}, adminEditFlags{isSelf: isSelf, isLastAdmin: isLastAdmin})
}

// IsAdminEditSelf reports whether the admin is viewing their own user detail page.
func IsAdminEditSelf(ctx context.Context) bool {
	f, _ := ctx.Value(ctxKeyAdminEditFlags{}).(adminEditFlags)
	return f.isSelf
}

// IsAdminEditLastAdmin reports whether the viewed user is the only remaining admin.
func IsAdminEditLastAdmin(ctx context.Context) bool {
	f, _ := ctx.Value(ctxKeyAdminEditFlags{}).(adminEditFlags)
	return f.isLastAdmin
}
