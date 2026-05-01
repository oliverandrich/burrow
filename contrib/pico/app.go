// Package pico provides a design system contrib app using PicoCSS v2.x and
// htmx. It embeds PicoCSS's default and 19 named accent variants and ships
// HTML layouts that can be used as the default layout for all pages. The
// layout is injected via middleware only when no other layout is already
// set in the request context.
//
// Pico is class-based ("classic"). For customization, set CSS custom
// properties in your own CSS file and pass it via [WithCustomCSS]:
//
//	pico.New(pico.WithCustomCSS("myapp/overrides.css"))
//
// The package is intentionally JS-free at the framework level — only the
// theme switcher script and htmx are loaded by the layout.
package pico

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/oliverandrich/burrow"
)

//go:embed static
var staticFS embed.FS

//go:embed templates
var templateFS embed.FS

// Color represents a PicoCSS accent color.
//
// [Default] selects the upstream default theme (pico.min.css). The named
// values correspond to the accent CSS files shipped by PicoCSS v2.x.
type Color string

// Color values supported by PicoCSS v2.x.
const (
	Default Color = ""
	Amber   Color = "amber"
	Blue    Color = "blue"
	Cyan    Color = "cyan"
	Fuchsia Color = "fuchsia"
	Green   Color = "green"
	Grey    Color = "grey"
	Indigo  Color = "indigo"
	Jade    Color = "jade"
	Lime    Color = "lime"
	Orange  Color = "orange"
	Pink    Color = "pink"
	Pumpkin Color = "pumpkin"
	Purple  Color = "purple"
	Red     Color = "red"
	Sand    Color = "sand"
	Slate   Color = "slate"
	Violet  Color = "violet"
	Yellow  Color = "yellow"
	Zinc    Color = "zinc"
)

// AllColors returns every supported [Color] including [Default]. Useful for
// tests and documentation.
func AllColors() []Color {
	return []Color{
		Default,
		Amber, Blue, Cyan, Fuchsia, Green, Grey, Indigo, Jade, Lime,
		Orange, Pink, Pumpkin, Purple, Red, Sand, Slate, Violet, Yellow, Zinc,
	}
}

// Option configures the pico app.
type Option func(*App)

// WithColor sets the accent color theme. Default is [Default] (PicoCSS's
// shipping default theme).
func WithColor(c Color) Option {
	return func(a *App) { a.color = c; a.customCSS = "" }
}

// WithCustomCSS sets a custom CSS file path (relative to staticfiles).
// This overrides [WithColor] and disables [WithCompactType] — when set,
// the custom file is the only stylesheet emitted. The CSS file must be
// served by the staticfiles app — either embedded in your app's static
// FS or in a contrib app.
//
//	pico.New(pico.WithCustomCSS("myapp/overrides.css"))
//
// The recommended customization workflow is to load PicoCSS's default
// stylesheet and override its CSS custom properties (such as
// --pico-primary or --pico-spacing) in a small project-specific file.
func WithCustomCSS(path string) Option {
	return func(a *App) { a.customCSS = path }
}

// WithCompactType ships an additional small stylesheet that flattens
// PicoCSS's responsive font-size scaling (capped at 106.25% on viewports
// ≥1024px instead of growing to 131.25% at 1536px) and tightens
// --pico-line-height to 1.4. Useful for admin/app UIs on large displays
// where Pico's blog-oriented defaults feel too heavy.
//
// Combines with [WithColor]. Ignored when [WithCustomCSS] is set.
func WithCompactType() Option {
	return func(a *App) { a.compactType = true }
}

// App implements a design system contrib app providing PicoCSS and htmx.
type App struct {
	color       Color
	customCSS   string
	compactType bool
}

// New creates a new pico design app with the given options.
func New(opts ...Option) *App {
	a := &App{color: Default}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *App) Name() string           { return "pico" }
func (a *App) Dependencies() []string { return []string{"staticfiles", "htmx"} }

// StaticFS returns the embedded static assets under the "pico" prefix.
func (a *App) StaticFS() (string, fs.FS) {
	sub, _ := fs.Sub(staticFS, "static")
	return "pico", sub
}

// TemplateFS returns the embedded HTML template files with the css.html
// template generated from the configured color theme or custom CSS path.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return &overlayFS{base: sub, cssHTML: a.cssTemplate()}
}

// cssTemplate returns the pico/css template content with the configured
// stylesheet links baked in. Emits one <link> for the primary stylesheet
// (custom CSS, color variant, or default), plus an additional <link> for
// the compact-type override when [WithCompactType] is set.
func (a *App) cssTemplate() string {
	primary := "pico/pico.min.css"
	switch {
	case a.customCSS != "":
		primary = a.customCSS
	case a.color != Default:
		primary = "pico/pico." + string(a.color) + ".min.css"
	}

	links := fmt.Sprintf(`<link rel="stylesheet" href="{{ staticURL %q }}">`, primary)
	if a.compactType && a.customCSS == "" {
		links += "\n" + fmt.Sprintf(`<link rel="stylesheet" href="{{ staticURL %q }}">`, "pico/pico-compact.min.css")
	}

	return fmt.Sprintf("{{ define \"pico/css\" -}}\n%s\n{{- end }}\n", links)
}

// Middleware returns middleware that injects the pico layout into the
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

// Layout returns the template name for the base pico layout (no navbar).
func Layout() string {
	return "pico/layout"
}

// NavLayout returns the template name for the pico layout with a navbar slot.
func NavLayout() string {
	return "pico/nav_layout"
}
