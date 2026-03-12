package htmx

import (
	"embed"
	"io/fs"

	"github.com/oliverandrich/burrow"
)

//go:embed static
var staticFS embed.FS

//go:embed templates
var templateFS embed.FS

// App implements a contrib app that serves the htmx JavaScript file
// and provides Go helpers for htmx request detection and response headers.
type App struct{}

// New creates a new htmx app.
func New() *App { return &App{} }

func (a *App) Name() string                       { return "htmx" }
func (a *App) Register(_ *burrow.AppConfig) error { return nil }
func (a *App) Dependencies() []string             { return []string{"staticfiles"} } //nolint:goconst

// TemplateFS returns the embedded HTML template files.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return sub
}

// StaticFS returns the embedded static assets under the "htmx" prefix.
func (a *App) StaticFS() (string, fs.FS) {
	sub, _ := fs.Sub(staticFS, "static")
	return "htmx", sub
}
