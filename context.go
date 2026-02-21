package burrow

import "context"

// ctxKeyNavItems is the context key for navigation items.
type ctxKeyNavItems struct{}

// WithContextValue returns a new context with the given key-value pair.
func WithContextValue(ctx context.Context, key, val any) context.Context {
	return context.WithValue(ctx, key, val) //nolint:staticcheck // framework key types are unexported
}

// ContextValue retrieves a typed value from the context.
func ContextValue[T any](ctx context.Context, key any) (T, bool) {
	val, ok := ctx.Value(key).(T)
	return val, ok
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
