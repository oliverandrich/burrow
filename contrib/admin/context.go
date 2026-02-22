package admin

import "context"

type ctxKeyNavGroups struct{}

// WithNavGroups stores nav groups in the context.
func WithNavGroups(ctx context.Context, groups []NavGroup) context.Context {
	return context.WithValue(ctx, ctxKeyNavGroups{}, groups)
}

// NavGroupsFromContext retrieves nav groups from the context.
func NavGroupsFromContext(ctx context.Context) []NavGroup {
	if groups, ok := ctx.Value(ctxKeyNavGroups{}).([]NavGroup); ok {
		return groups
	}
	return nil
}

type ctxKeyRequestPath struct{}

// WithRequestPath stores the current request path in the context.
func WithRequestPath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestPath{}, path)
}

// RequestPathFromContext retrieves the current request path from the context.
func RequestPathFromContext(ctx context.Context) string {
	if path, ok := ctx.Value(ctxKeyRequestPath{}).(string); ok {
		return path
	}
	return ""
}
