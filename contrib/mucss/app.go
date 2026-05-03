// Package mucss provides a design system contrib app using µCSS v1.4.7.
// µCSS is built on PicoCSS v2 with the upstream Pico bugs fixed and
// additional components (Hero, Alert, Toast, Modal, Pagination, Badge,
// Skeleton, Spinner, Tabs, Accordion) shipped as part of the framework.
//
// µCSS is class-based ("classic"). For customization, set CSS custom
// properties in your own CSS file and pass it via [WithCustomCSS]:
//
//	mucss.New(mucss.WithCustomCSS("myapp/overrides.css"))
//
// Customization variables use the --mu- prefix (renamed from upstream
// Pico's --pico-). The package is JS-free at the framework level.
package mucss

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/bsicons"
	"github.com/urfave/cli/v3"
)

//go:embed static
var staticFS embed.FS

//go:embed templates
var templateFS embed.FS

// Color represents a µCSS accent color.
//
// [Default] selects the upstream default theme (mu.css). The named values
// correspond to the accent CSS files shipped by µCSS v1.x.
type Color string

// Color values supported by µCSS v1.x.
const (
	Default Color = ""
	Amber   Color = "amber"
	Azure   Color = "azure"
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
		Amber, Azure, Blue, Cyan, Fuchsia, Green, Grey, Indigo, Jade, Lime,
		Orange, Pink, Pumpkin, Purple, Red, Sand, Slate, Violet, Yellow, Zinc,
	}
}

// Option configures the mucss app.
type Option func(*App)

// WithColor sets the accent color theme. Default is [Default] (µCSS's
// shipping default theme).
func WithColor(c Color) Option {
	return func(a *App) { a.color = c; a.customCSS = "" }
}

// WithCustomCSS sets a custom CSS file path (relative to staticfiles).
// This overrides [WithColor], disables [WithCompactType], and disables
// the always-on Burrow extras — when set, the custom file is the only
// stylesheet emitted. The CSS file must be served by the staticfiles
// app — either embedded in your app's static FS or in a contrib app.
//
//	mucss.New(mucss.WithCustomCSS("myapp/overrides.css"))
//
// The recommended customization workflow is to load µCSS's default
// stylesheet and override its CSS custom properties (such as
// --mu-primary or --mu-spacing) in a small project-specific file.
func WithCustomCSS(path string) Option {
	return func(a *App) { a.customCSS = path }
}

// WithCompactType ships an additional small stylesheet that flattens
// µCSS's responsive font-size scaling and tightens spacing for
// admin/app UIs on large displays where µCSS's blog-oriented defaults
// (inherited from PicoCSS) feel too heavy.
//
// Globally:
//   - --mu-line-height: 1.4 (default 1.5)
//
// On viewports ≥1024px:
//   - --mu-font-size: 106.25% (default grows to 131.25% at 1536px)
//   - --mu-spacing: 0.85rem (default 1rem)
//   - --mu-typography-spacing-vertical: 0.75rem (default 1rem)
//   - --mu-form-element-spacing-vertical: 0.5rem (default 0.75rem)
//   - --mu-form-element-spacing-horizontal: 0.75rem (default 1rem)
//   - --mu-grid-column-gap: 0.75rem (default 1rem)
//   - --mu-grid-row-gap: 0.75rem (default 1rem)
//
// Navbar spacing is intentionally left at the µCSS default — compact
// typography is meant to tighten content, not the chrome.
//
// Mobile/tablet defaults are unchanged so touch targets stay within
// recommended sizes.
//
// Combines with [WithColor]. Ignored when [WithCustomCSS] is set.
func WithCompactType() Option {
	return func(a *App) { a.compactType = true }
}

// App implements a design system contrib app providing µCSS and htmx.
type App struct {
	color       Color
	customCSS   string
	compactType bool
}

// Compile-time guarantees that App satisfies the burrow optional
// interfaces it claims to implement. Renaming or removing any of these
// from the framework would surface here as a build error rather than
// at runtime when Burrow's registry walks the app.
var (
	_ burrow.App             = (*App)(nil)
	_ burrow.HasDependencies = (*App)(nil)
	_ burrow.Configurable    = (*App)(nil)
	_ burrow.HasStaticFiles  = (*App)(nil)
	_ burrow.HasTemplates    = (*App)(nil)
	_ burrow.HasMiddleware   = (*App)(nil)
)

// New creates a new mucss design app with the given options.
func New(opts ...Option) *App {
	a := &App{color: Default}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *App) Name() string           { return "mucss" }
func (a *App) Dependencies() []string { return []string{"staticfiles", "htmx"} }

// Configure registers the icon template functions used by the
// theme_switcher template. Apps using their own templates with mucss
// can rely on these names being registered.
func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
	cfg.RegisterIconFunc("iconSunFill", bsicons.SunFill)
	cfg.RegisterIconFunc("iconMoonStarsFill", bsicons.MoonStarsFill)
	cfg.RegisterIconFunc("iconCircleHalf", bsicons.CircleHalf)
	return nil
}

// StaticFS returns the embedded static assets under the "mucss" prefix.
func (a *App) StaticFS() (string, fs.FS) {
	sub, _ := fs.Sub(staticFS, "static")
	return "mucss", sub
}

// TemplateFS returns the embedded HTML template files with the css.html
// template generated from the configured color theme or custom CSS path.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return &overlayFS{base: sub, cssHTML: a.cssTemplate()}
}

// cssTemplate returns the mucss/css template content with the configured
// stylesheet links baked in. When [WithCustomCSS] is set, only the custom
// stylesheet is emitted. Otherwise: primary stylesheet (default or
// [WithColor] variant) + always-on Burrow extras (navbar-dropdown polish)
// + optional compact-type override when [WithCompactType] is set.
func (a *App) cssTemplate() string {
	if a.customCSS != "" {
		return fmt.Sprintf("{{ define \"mucss/css\" -}}\n<link rel=\"stylesheet\" href=\"{{ staticURL %q }}\">\n{{- end }}\n", a.customCSS)
	}

	primary := "mucss/mu.css"
	if a.color != Default {
		primary = "mucss/mu." + string(a.color) + ".css"
	}

	links := fmt.Sprintf(`<link rel="stylesheet" href="{{ staticURL %q }}">`, primary)
	links += "\n" + fmt.Sprintf(`<link rel="stylesheet" href="{{ staticURL %q }}">`, "mucss/mu-extras.min.css")
	if a.compactType {
		links += "\n" + fmt.Sprintf(`<link rel="stylesheet" href="{{ staticURL %q }}">`, "mucss/mu-compact.min.css")
	}

	return fmt.Sprintf("{{ define \"mucss/css\" -}}\n%s\n{{- end }}\n", links)
}

// Middleware returns middleware that injects the mucss layout into the
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

// Layout returns the template name for the base mucss layout (no navbar).
func Layout() string {
	return "mucss/layout"
}

// NavLayout returns the template name for the mucss layout with a navbar slot.
func NavLayout() string {
	return "mucss/nav_layout"
}
