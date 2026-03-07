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

// UserFromContext retrieves the authenticated user from the context.
func UserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value(ctxKeyUser{}).(*User); ok {
		return user
	}
	return nil
}

// IsAuthenticated returns true if a user is present in the context.
func IsAuthenticated(ctx context.Context) bool {
	return UserFromContext(ctx) != nil
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

// LogoFromContext retrieves the logo HTML from context, or empty if not set.
func LogoFromContext(ctx context.Context) template.HTML {
	logo, _ := ctx.Value(ctxKeyLogo{}).(template.HTML)
	return logo
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
