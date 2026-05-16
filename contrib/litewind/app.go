// Package litewind provides a Tailwind-utility-based design contrib backed
// by a vendored static CSS (Burrow's custom Litewind build). One <link>,
// no build step out of the box. Production-side CSS optimization is the
// optional companion tool cmd/burrow-purge.
//
// Usage:
//
//	srv := burrow.NewServer(
//	    staticfiles.New(),
//	    htmx.New(),
//	    litewind.New(),
//	    // ...
//	)
//
// Theme-switcher (auto/light/dark) ports the data-theme convention from
// contrib/mucss. The vendored CSS is intended to ship dark: variants
// targeting [data-theme="dark"]; until the custom build lands, the file
// at static/litewind.min.css is a placeholder — see bean tiv8.
package litewind

import (
	"embed"
	"io/fs"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/bsicons"
	"github.com/urfave/cli/v3"
)

//go:embed static
var staticFS embed.FS

//go:embed templates
var templateFS embed.FS

// App is the Burrow contrib app providing a vendored Litewind CSS and
// the data-theme switcher templates.
type App struct{}

// Compile-time guarantees that App satisfies the burrow optional
// interfaces it claims to implement.
var (
	_ burrow.App             = (*App)(nil)
	_ burrow.HasDependencies = (*App)(nil)
	_ burrow.Configurable    = (*App)(nil)
	_ burrow.HasStaticFiles  = (*App)(nil)
	_ burrow.HasTemplates    = (*App)(nil)
)

// New creates a new litewind app.
func New() *App {
	return &App{}
}

func (a *App) Name() string           { return "litewind" }
func (a *App) Dependencies() []string { return []string{"staticfiles", "htmx"} }

// Configure registers the icon template functions used by the
// theme_switcher template. Apps using their own templates with litewind
// can rely on these names being registered.
func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
	cfg.RegisterIconFunc("iconSunFill", bsicons.SunFill)
	cfg.RegisterIconFunc("iconMoonStarsFill", bsicons.MoonStarsFill)
	cfg.RegisterIconFunc("iconCircleHalf", bsicons.CircleHalf)
	return nil
}

// StaticFS returns the embedded static assets under the "litewind" prefix.
func (a *App) StaticFS() (string, fs.FS) {
	sub, _ := fs.Sub(staticFS, "static")
	return "litewind", sub
}

// TemplateFS returns the embedded HTML template files (css, theme_script,
// theme_switcher).
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return sub
}
