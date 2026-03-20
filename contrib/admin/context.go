package admin

import "context"

type ctxKeyNavGroups struct{}

// WithNavGroups stores nav groups in the context.
func WithNavGroups(ctx context.Context, groups []NavGroup) context.Context {
	return context.WithValue(ctx, ctxKeyNavGroups{}, groups)
}

// NavGroups retrieves nav groups from the context.
func NavGroups(ctx context.Context) []NavGroup {
	if groups, ok := ctx.Value(ctxKeyNavGroups{}).([]NavGroup); ok {
		return groups
	}
	return nil
}

// NavGroupsFromContext is a deprecated alias for [NavGroups].
//
//go:fix inline
func NavGroupsFromContext(ctx context.Context) []NavGroup {
	return NavGroups(ctx)
}

type ctxKeyRequestPath struct{}

// WithRequestPath stores the current request path in the context.
func WithRequestPath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestPath{}, path)
}

// RequestPath retrieves the current request path from the context.
func RequestPath(ctx context.Context) string {
	if path, ok := ctx.Value(ctxKeyRequestPath{}).(string); ok {
		return path
	}
	return ""
}

// RequestPathFromContext is a deprecated alias for [RequestPath].
//
//go:fix inline
func RequestPathFromContext(ctx context.Context) string {
	return RequestPath(ctx)
}
