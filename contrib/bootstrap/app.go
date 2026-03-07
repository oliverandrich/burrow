// Package bootstrap provides a design system contrib app using Bootstrap 5,
// Bootstrap Icons, and htmx. It embeds static assets and provides a base HTML
// layout that can be used as the default layout for all pages. The layout is
// injected via middleware only when no other layout is already set in the
// request context.
package bootstrap

import (
	"embed"
	"html/template"
	"io/fs"
	"maps"
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/bsicons"
)

//go:embed static
var staticFS embed.FS

//go:embed templates
var templateFS embed.FS

// App implements a design system contrib app providing Bootstrap CSS/JS and htmx.
type App struct{}

// New creates a new bootstrap design app.
func New() *App { return &App{} }

func (a *App) Name() string                       { return "bootstrap" }
func (a *App) Register(_ *burrow.AppConfig) error { return nil }
func (a *App) Dependencies() []string             { return []string{"staticfiles"} } //nolint:goconst

// StaticFS returns the embedded static assets under the "bootstrap" prefix.
func (a *App) StaticFS() (string, fs.FS) {
	sub, _ := fs.Sub(staticFS, "static")
	return "bootstrap", sub
}

// TemplateFS returns the embedded HTML template files.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return sub
}

// FuncMap returns template functions provided by the bootstrap app.
func (a *App) FuncMap() template.FuncMap {
	return template.FuncMap{
		"iconSunFill":       func() template.HTML { return bsicons.SunFill() },
		"iconMoonStarsFill": func() template.HTML { return bsicons.MoonStarsFill() },
		"iconCircleHalf":    func() template.HTML { return bsicons.CircleHalf() },
		"pageURL":           pageURL,
		"pageLimit":         pageLimit,
		"pageNumbers":       pageNumbers,
		"add":               func(a, b int) int { return a + b },
		"sub":               func(a, b int) int { return a - b },
	}
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

// Layout returns a LayoutFunc that renders page content inside the
// bootstrap/layout template.
func Layout() burrow.LayoutFunc {
	return func(w http.ResponseWriter, r *http.Request, code int, content template.HTML, data map[string]any) error {
		exec := burrow.TemplateExecutorFromContext(r.Context())
		if exec == nil {
			return burrow.HTML(w, code, string(content))
		}

		layoutData := make(map[string]any, len(data)+2)
		maps.Copy(layoutData, data)
		layoutData["Content"] = content
		if _, ok := layoutData["Title"]; !ok {
			layoutData["Title"] = ""
		}

		html, err := exec(r, "bootstrap/layout", layoutData)
		if err != nil {
			return err
		}
		return burrow.HTML(w, code, string(html))
	}
}
