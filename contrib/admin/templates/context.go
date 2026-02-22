package templates

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
