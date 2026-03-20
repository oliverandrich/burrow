// Package auth provides authentication as a burrow contrib app.
//
// It implements WebAuthn (passkeys), recovery codes, email verification,
// and invite-only registration. Context helpers provide access to the
// authenticated user from any handler.
package auth

import (
	"context"
	"html/template"
)

// ctxKeyUser is the context key for the authenticated user.
type ctxKeyUser struct{}

// CurrentUser retrieves the authenticated user from the context.
func CurrentUser(ctx context.Context) *User {
	if user, ok := ctx.Value(ctxKeyUser{}).(*User); ok {
		return user
	}
	return nil
}

// UserFromContext is a deprecated alias for [CurrentUser].
//
//go:fix inline
func UserFromContext(ctx context.Context) *User {
	return CurrentUser(ctx)
}

// IsAuthenticated returns true if a user is present in the context.
func IsAuthenticated(ctx context.Context) bool {
	return CurrentUser(ctx) != nil
}

// WithUser returns a new context with the user set.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, ctxKeyUser{}, user)
}

// ctxKeyLogo is the context key for the optional auth page logo component.
type ctxKeyLogo struct{}

// WithLogo returns a new context with the logo HTML set.
func WithLogo(ctx context.Context, logo template.HTML) context.Context {
	return context.WithValue(ctx, ctxKeyLogo{}, logo)
}

// Logo retrieves the logo HTML from context, or empty if not set.
func Logo(ctx context.Context) template.HTML {
	logo, _ := ctx.Value(ctxKeyLogo{}).(template.HTML)
	return logo
}

// LogoFromContext is a deprecated alias for [Logo].
//
//go:fix inline
func LogoFromContext(ctx context.Context) template.HTML {
	return Logo(ctx)
}
