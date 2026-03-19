// Package bootstrap provides a design system contrib app using Bootstrap 5,
// Bootstrap Icons, and htmx. It embeds static assets and provides HTML
// layouts that can be used as the default layout for all pages. The layout is
// injected via middleware only when no other layout is already set in the
// request context.
//
// The app ships three color themes (blue, purple, gray) selectable via
// [WithColor].
package bootstrap

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/bsicons"
)

//go:embed static
var staticFS embed.FS

//go:embed templates
var templateFS embed.FS

// Color represents a Bootstrap color theme.
type Color string

const (
	Default Color = ""
	Blue    Color = "blue"
	Purple  Color = "purple"
	Gray    Color = "gray"
)

// Option configures the bootstrap app.
type Option func(*App)

// WithColor sets the color theme. Default is [Purple].
func WithColor(c Color) Option {
	return func(a *App) { a.color = c; a.customCSS = "" }
}

// WithCustomCSS sets a custom CSS file path (relative to staticfiles).
// This overrides [WithColor]. The CSS file must be served by the staticfiles
// app — either embedded in your app's static FS or in a contrib app.
//
//	bootstrap.New(bootstrap.WithCustomCSS("myapp/mytheme.min.css"))
func WithCustomCSS(path string) Option {
	return func(a *App) { a.customCSS = path }
}

// App implements a design system contrib app providing Bootstrap CSS/JS and htmx.
type App struct {
	color     Color
	customCSS string
}

// New creates a new bootstrap design app with the given options.
func New(opts ...Option) *App {
	a := &App{
		color: Purple,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *App) Name() string           { return "bootstrap" }
func (a *App) Dependencies() []string { return []string{"staticfiles", "htmx"} } //nolint:goconst

func (a *App) Register(cfg *burrow.AppConfig) error {
	cfg.RegisterIconFunc("iconSunFill", bsicons.SunFill)
	cfg.RegisterIconFunc("iconMoonStarsFill", bsicons.MoonStarsFill)
	cfg.RegisterIconFunc("iconCircleHalf", bsicons.CircleHalf)
	return nil
}

// StaticFS returns the embedded static assets under the "bootstrap" prefix.
func (a *App) StaticFS() (string, fs.FS) {
	sub, _ := fs.Sub(staticFS, "static")
	return "bootstrap", sub
}

// TemplateFS returns the embedded HTML template files with the CSS link
// template generated from the configured color theme.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return &overlayFS{base: sub, cssHTML: a.cssTemplate()}
}

// cssTemplate returns the bootstrap/css template content with the configured
// CSS path baked in.
func (a *App) cssTemplate() string {
	path := "bootstrap/bootstrap.min.css"
	if a.customCSS != "" {
		path = a.customCSS
	} else if a.color != Default {
		path = "bootstrap/theme-" + string(a.color) + ".min.css"
	}
	return fmt.Sprintf(`{{ define "bootstrap/css" -}}
<link rel="stylesheet" href="{{ staticURL %q }}">
{{- end }}
`, path)
}

// Middleware returns middleware that injects the bootstrap layout into the
// request context when no layout is already set.
func (a *App) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if burrow.Layout(r.Context()) == "" {
					ctx := burrow.WithLayout(r.Context(), Layout())
					r = r.WithContext(ctx)
				}
				next.ServeHTTP(w, r)
			})
		},
	}
}

// Layout returns the template name for the base bootstrap layout (no navbar).
func Layout() string {
	return "bootstrap/layout"
}

// NavLayout returns the template name for the bootstrap layout with navbar slot.
func NavLayout() string {
	return "bootstrap/nav_layout"
}
