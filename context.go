package burrow

import (
	"context"
	"html/template"
	"net/http"
)

// TemplateExecutor executes a named template with the given data and returns
// the rendered HTML. It is stored in the request context by the template
// middleware and used by RenderTemplate.
type TemplateExecutor func(r *http.Request, name string, data map[string]any) (template.HTML, error)

// AuthChecker provides authentication and authorization checks via closures.
// This allows the core framework to filter nav items by auth state without
// importing contrib/auth. Auth apps inject an AuthChecker into the context;
// the framework reads it when building NavLinks.
type AuthChecker struct {
	IsAuthenticated func() bool
	IsAdmin         func() bool
}

// Context key types for framework-provided values.
type (
	ctxKeyLayout           struct{}
	ctxKeyNavItems         struct{}
	ctxKeyTemplateExecutor struct{}
	ctxKeyAuthChecker      struct{}
)

// WithContextValue returns a new context with the given key-value pair.
// This is a convenience wrapper around [context.WithValue] used primarily
// by contrib app authors to store app-specific values in the request context.
// Application developers typically use typed helpers like [WithLayout] or
// contrib-specific functions (e.g. csrf.WithToken) instead.
func WithContextValue(ctx context.Context, key, val any) context.Context {
	return context.WithValue(ctx, key, val) //nolint:staticcheck // framework key types are unexported
}

// ContextValue retrieves a typed value from the context.
// It is the generic counterpart to [WithContextValue], used by contrib app
// authors to read back app-specific context values with type safety.
func ContextValue[T any](ctx context.Context, key any) (T, bool) {
	val, ok := ctx.Value(key).(T)
	return val, ok
}

// WithLayout stores the layout template name in the context.
func WithLayout(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, ctxKeyLayout{}, name)
}

// Layout retrieves the layout template name from the context.
func Layout(ctx context.Context) string {
	if name, ok := ctx.Value(ctxKeyLayout{}).(string); ok {
		return name
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

// WithTemplateExecutor stores the template executor in the context.
func WithTemplateExecutor(ctx context.Context, exec TemplateExecutor) context.Context {
	return context.WithValue(ctx, ctxKeyTemplateExecutor{}, exec)
}

// TemplateExecutorFromContext retrieves the template executor from the context.
func TemplateExecutorFromContext(ctx context.Context) TemplateExecutor {
	if exec, ok := ctx.Value(ctxKeyTemplateExecutor{}).(TemplateExecutor); ok {
		return exec
	}
	return nil
}

// WithAuthChecker stores an AuthChecker in the context. This is typically
// called by auth middleware to make authentication state available to the
// core template functions without an import cycle.
func WithAuthChecker(ctx context.Context, checker AuthChecker) context.Context {
	return context.WithValue(ctx, ctxKeyAuthChecker{}, checker)
}

// AuthCheckerContextKey returns the context key used for AuthChecker storage.
// This is intended for testing: use ContextValue with this key to inspect
// the AuthChecker set by middleware.
func AuthCheckerContextKey() any {
	return ctxKeyAuthChecker{}
}

// isAuthenticated returns true if the AuthChecker in context reports
// authentication. Returns false if no AuthChecker is set.
func isAuthenticated(ctx context.Context) bool {
	if checker, ok := ctx.Value(ctxKeyAuthChecker{}).(AuthChecker); ok {
		return checker.IsAuthenticated()
	}
	return false
}

// isAdmin returns true if the AuthChecker in context reports admin status.
// Returns false if no AuthChecker is set.
func isAdmin(ctx context.Context) bool {
	if checker, ok := ctx.Value(ctxKeyAuthChecker{}).(AuthChecker); ok {
		return checker.IsAdmin()
	}
	return false
}
