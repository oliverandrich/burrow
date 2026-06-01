// Package auth provides authentication as a burrow contrib app.
//
// It implements WebAuthn (passkeys), recovery codes, email verification,
// and invite-only registration. Context helpers provide access to the
// authenticated user from any handler.
package auth

import (
	"context"
	"html/template"

	"github.com/oliverandrich/burrow"
)

// ctxKeyUser is the context key for the authenticated user. The value
// stored is a *User[P] boxed as any — type-erased so middleware can stay
// non-generic. [CurrentUser] re-types it via a type parameter at the
// access site.
type ctxKeyUser struct{}

// CurrentUser retrieves the authenticated user from the context. The
// Profile type parameter must match the type registered with [auth.New].
// Returns nil if no user is on the context, or if the type parameter
// disagrees with what middleware stored.
func CurrentUser[P any](ctx context.Context) *User[P] {
	if user, ok := ctx.Value(ctxKeyUser{}).(*User[P]); ok {
		return user
	}
	return nil
}

// MustCurrentUser returns the authenticated user from the context.
// It panics if no user is present — only use in handlers protected by [RequireAuth] middleware.
func MustCurrentUser[P any](ctx context.Context) *User[P] {
	user := CurrentUser[P](ctx)
	if user == nil {
		panic("auth: MustCurrentUser called without authenticated user — is RequireAuth middleware applied?")
	}
	return user
}

// IsAuthenticated returns true if a user is present in the context. This
// stays non-generic by checking the context value's presence without
// caring about the Profile type.
func IsAuthenticated(ctx context.Context) bool {
	return ctx.Value(ctxKeyUser{}) != nil
}

// WithUser returns a new context with the user set. The user argument is
// typed as any so middleware and other type-erased layers don't need to
// know the Profile type — only the [CurrentUser] read site does.
func WithUser(ctx context.Context, user any) context.Context {
	return context.WithValue(ctx, ctxKeyUser{}, user)
}

// setAuthContext stamps an authenticated user into the request context: both
// the user value (via [WithUser]) and a [burrow.AuthChecker] for the core
// navLinks / role-gating template helpers. Both the session middleware and
// the API-key middleware funnel through here, so the authenticated-context
// contract has a single source.
func setAuthContext[P any](ctx context.Context, user *User[P]) context.Context {
	ctx = WithUser(ctx, user)
	return burrow.WithAuthChecker(ctx, burrow.AuthChecker{
		IsAuthenticated: func() bool { return true },
		IsStaff:         func() bool { return user.IsStaff() },
		IsAdmin:         func() bool { return user.IsAdmin() },
	})
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
