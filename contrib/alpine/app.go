// Package alpine provides Alpine.js as a burrow contrib app.
// It embeds the Alpine.js JavaScript bundle and serves it via the
// staticfiles app with content-hashed URLs and immutable caching.
package alpine

import (
	"embed"
	"io/fs"

	"github.com/oliverandrich/burrow"
)

//go:embed static
var staticFS embed.FS

//go:embed templates
var templateFS embed.FS

// App implements a contrib app that serves the Alpine.js JavaScript file.
type App struct{}

// New creates a new Alpine.js app.
func New() *App { return &App{} }

func (a *App) Name() string                       { return "alpine" }
func (a *App) Register(_ *burrow.AppConfig) error { return nil }
func (a *App) Dependencies() []string             { return []string{"staticfiles"} } //nolint:goconst

// TemplateFS returns the embedded HTML template files.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return sub
}

// StaticFS returns the embedded static assets under the "alpine" prefix.
func (a *App) StaticFS() (string, fs.FS) {
	sub, _ := fs.Sub(staticFS, "static")
	return "alpine", sub
}
