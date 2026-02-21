package burrow

import "context"

// Context key types for framework-provided values.
type (
	ctxKeyLayout   struct{}
	ctxKeyNavItems struct{}
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
