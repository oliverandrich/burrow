// Package app provides the layout and homepage for the tutorial application.
package app

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// App provides the layout, homepage, and the project's stylesheet.
type App struct{}

// New creates the shell app.
func New() *App { return &App{} }

func (a *App) Name() string { return "app" }

func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return sub
}

// StaticFS contributes app.css under the "app" prefix so the layout can
// link to it via `{{ staticURL "app/app.css" }}`.
func (a *App) StaticFS() (string, fs.FS) {
	sub, _ := fs.Sub(staticFS, "static")
	return "app", sub
}

func (a *App) NavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{Label: "Home", URL: "/", Position: 0},
	}
}

func (a *App) Routes(r chi.Router) {
	r.Get("/", burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return burrow.Render(w, r, http.StatusOK, "pages/home", map[string]any{
			"Title": "Welcome to Polls",
		})
	}))
}

// Layout returns the template name for the app layout.
func Layout() string {
	return "app/layout"
}
