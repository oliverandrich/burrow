package admin

import (
	"context"

	"github.com/oliverandrich/burrow"
)

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

// WithRequestPath is a deprecated alias for [burrow.WithRequestPath].
//
//go:fix inline
func WithRequestPath(ctx context.Context, path string) context.Context {
	return burrow.WithRequestPath(ctx, path)
}

// RequestPath is a deprecated alias for [burrow.RequestPath].
//
//go:fix inline
func RequestPath(ctx context.Context) string {
	return burrow.RequestPath(ctx)
}

// RequestPathFromContext is a deprecated alias for [burrow.RequestPath].
//
//go:fix inline
func RequestPathFromContext(ctx context.Context) string {
	return burrow.RequestPath(ctx)
}
