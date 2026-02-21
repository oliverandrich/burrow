// Package admin provides an admin panel coordinator as a burrow contrib app.
// It discovers admin views from other apps via the HasAdmin interface.
package admin

import (
	"context"

	"codeberg.org/oliverandrich/burrow"
)

// Context key types for admin-provided values.
type (
	ctxKeyAdminNavItems struct{}
	ctxKeyAdminLayout   struct{}
)

// WithNavItems stores admin navigation items in the context.
func WithNavItems(ctx context.Context, items []burrow.NavItem) context.Context {
	return context.WithValue(ctx, ctxKeyAdminNavItems{}, items)
}

// NavItems retrieves the admin navigation items from the context.
func NavItems(ctx context.Context) []burrow.NavItem {
	if items, ok := ctx.Value(ctxKeyAdminNavItems{}).([]burrow.NavItem); ok {
		return items
	}
	return nil
}

// WithLayout stores the admin layout function in the context.
func WithLayout(ctx context.Context, fn burrow.LayoutFunc) context.Context {
	return context.WithValue(ctx, ctxKeyAdminLayout{}, fn)
}

// Layout retrieves the admin layout function from the context.
func Layout(ctx context.Context) burrow.LayoutFunc {
	if fn, ok := ctx.Value(ctxKeyAdminLayout{}).(burrow.LayoutFunc); ok {
		return fn
	}
	return nil
}
