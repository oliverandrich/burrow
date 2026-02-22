// Package layout provides the example app's HTML layout with a Bootstrap 5
// navbar, auth-aware navigation, and static asset URLs.
package layout

import (
	"context"
	"net/http"
	"strings"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
)

// ctxKeyRequestPath is used to pass the request path into the template context.
type ctxKeyRequestPath struct{}

// Layout returns a LayoutFunc that wraps page content in the app layout.
func Layout() burrow.LayoutFunc {
	return layout
}

// Middleware returns middleware that stores the request path in context
// so templates can highlight the active nav link.
func Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxKeyRequestPath{}, r.URL.Path)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// visibleNavItems returns nav items the current user should see.
func visibleNavItems(ctx context.Context) []burrow.NavItem {
	user := auth.UserFromContext(ctx)
	var visible []burrow.NavItem
	for _, item := range burrow.NavItems(ctx) {
		if item.AuthOnly && user == nil {
			continue
		}
		if item.AdminOnly && (user == nil || !user.IsAdmin()) {
			continue
		}
		visible = append(visible, item)
	}
	return visible
}

// navLinkClass returns CSS classes for a nav link, marking it active
// when it matches the current path.
func navLinkClass(ctx context.Context, itemURL string) string {
	current, _ := ctx.Value(ctxKeyRequestPath{}).(string)
	if current == "" {
		return "nav-link"
	}
	if itemURL == "/" {
		if current == "/" {
			return "nav-link active"
		}
		return "nav-link"
	}
	if strings.HasPrefix(current, itemURL) {
		return "nav-link active"
	}
	return "nav-link"
}
