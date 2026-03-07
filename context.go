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

// LayoutFunc wraps page content in a layout template.
// The layout reads framework values (nav items, user, locale, CSRF)
// from the request context via helper functions.
type LayoutFunc func(w http.ResponseWriter, r *http.Request, code int, content template.HTML, data map[string]any) error

// Context key types for framework-provided values.
type (
	ctxKeyLayout           struct{}
	ctxKeyNavItems         struct{}
	ctxKeyTemplateExecutor struct{}
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

// WithLayout stores the app layout function in the context.
func WithLayout(ctx context.Context, fn LayoutFunc) context.Context {
	return context.WithValue(ctx, ctxKeyLayout{}, fn)
}

// Layout retrieves the app layout function from the context.
func Layout(ctx context.Context) LayoutFunc {
	if fn, ok := ctx.Value(ctxKeyLayout{}).(LayoutFunc); ok {
		return fn
	}
	return nil
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
