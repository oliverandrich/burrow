// Package pages provides the layout and homepage for the tutorial application.
package pages

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
)

//go:embed templates
var templateFS embed.FS

// App provides the layout and homepage.
type App struct{}

// New creates a new pages app.
func New() *App { return &App{} }

func (a *App) Name() string                       { return "pages" }
func (a *App) Register(_ *burrow.AppConfig) error { return nil }

func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return sub
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
