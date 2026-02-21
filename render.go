package burrow

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
)

// Render renders a templ.Component into the HTTP response with the given status code.
func Render(w http.ResponseWriter, r *http.Request, statusCode int, component templ.Component) error {
	buf := templ.GetBuffer()
	defer templ.ReleaseBuffer(buf)

	if err := component.Render(r.Context(), buf); err != nil {
		return err
	}

	return HTML(w, statusCode, buf.String())
}

// LayoutFunc wraps page content in a layout template.
// The layout reads framework values (nav items, user, locale, CSRF)
// from the context via helper functions.
type LayoutFunc func(title string, content templ.Component) templ.Component

// Layouts holds the layout functions for different areas of the application.
type Layouts struct {
	App   LayoutFunc // User-facing pages (login, dashboard, etc.)
	Admin LayoutFunc // Admin pages (user management, etc.)
}

// Context key types for framework-provided values.
type (
	ctxKeyCSRFToken     struct{}
	ctxKeyNavItems      struct{}
	ctxKeyAdminNavItems struct{}
)

// WithContextValue returns a new context with the given key-value pair.
func WithContextValue(ctx context.Context, key, val any) context.Context {
	return context.WithValue(ctx, key, val) //nolint:staticcheck // framework key types are unexported
}

// ContextValue retrieves a typed value from the context.
func ContextValue[T any](ctx context.Context, key any) (T, bool) {
	val, ok := ctx.Value(key).(T)
	return val, ok
}

// WithCSRFToken stores a CSRF token in the context.
func WithCSRFToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, ctxKeyCSRFToken{}, token)
}

// CSRFToken retrieves the CSRF token from the context.
func CSRFToken(ctx context.Context) string {
	if token, ok := ctx.Value(ctxKeyCSRFToken{}).(string); ok {
		return token
	}
	return ""
}

// WithNavItems stores navigation items in the context.
func WithNavItems(ctx context.Context, items []NavItem) context.Context {
	return context.WithValue(ctx, ctxKeyNavItems{}, items)
}

// NavItems retrieves the navigation items from the context.
func NavItems(ctx context.Context) []NavItem {
	if items, ok := ctx.Value(ctxKeyNavItems{}).([]NavItem); ok {
		return items
	}
	return nil
}

// WithAdminNavItems stores admin navigation items in the context.
func WithAdminNavItems(ctx context.Context, items []NavItem) context.Context {
	return context.WithValue(ctx, ctxKeyAdminNavItems{}, items)
}

// AdminNavItems retrieves the admin navigation items from the context.
func AdminNavItems(ctx context.Context) []NavItem {
	if items, ok := ctx.Value(ctxKeyAdminNavItems{}).([]NavItem); ok {
		return items
	}
	return nil
}
