// Package admin provides an admin panel coordinator as a burrow contrib app.
// It discovers admin views from other apps via the HasAdmin interface.
package admin

import (
	"context"

	"codeberg.org/oliverandrich/burrow"
)

// ctxKeyAdminNavItems is the context key for admin navigation items.
type ctxKeyAdminNavItems struct{}

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
