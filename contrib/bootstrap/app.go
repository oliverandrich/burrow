// Package bootstrap provides a design system contrib app using Bootstrap 5,
// Bootstrap Icons, and htmx. It embeds static assets and provides a base HTML
// layout that can be used as the default layout for all pages. The layout is
// injected via middleware only when no other layout is already set in the
// request context.
package bootstrap

import (
	"embed"
	"io/fs"
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/bootstrap/templates"
)

//go:embed static
var staticFS embed.FS

// App implements a design system contrib app providing Bootstrap CSS/JS and htmx.
type App struct{}

// New creates a new bootstrap design app.
func New() *App { return &App{} }

func (a *App) Name() string                       { return "bootstrap" }
func (a *App) Register(_ *burrow.AppConfig) error { return nil }

// StaticFS returns the embedded static assets under the "bootstrap" prefix.
func (a *App) StaticFS() (string, fs.FS) {
	sub, _ := fs.Sub(staticFS, "static")
	return "bootstrap", sub
}

// Middleware returns middleware that injects the bootstrap layout into the
// request context when no layout is already set. This makes bootstrap the
// default layout while allowing srv.SetLayout() or route-level overrides
// to take precedence.
func (a *App) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if burrow.Layout(r.Context()) == nil {
					ctx := burrow.WithLayout(r.Context(), Layout())
					r = r.WithContext(ctx)
				}
				next.ServeHTTP(w, r)
			})
		},
	}
}

// Layout returns a LayoutFunc using Bootstrap 5 and htmx.
func Layout() burrow.LayoutFunc {
	return templates.Layout
}
