// Package pages provides the example app's static pages (homepage),
// layout rendering, and icon template functions.
package pages

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/bsicons"
	"github.com/oliverandrich/burrow/contrib/messages"
)

//go:embed translations
var translationFS embed.FS

//go:embed templates
var templateFS embed.FS

// App implements the pages app.
type App struct{}

// New creates a new pages app.
func New() *App { return &App{} }

func (a *App) Name() string { return "pages" }

func (a *App) Register(cfg *burrow.AppConfig) error {
	cfg.RegisterIconFunc("iconHouse", bsicons.House)
	cfg.RegisterIconFunc("iconKey", bsicons.Key)
	cfg.RegisterIconFunc("iconPuzzle", bsicons.Puzzle)
	cfg.RegisterIconFunc("iconLightning", bsicons.Lightning)
	cfg.RegisterIconFunc("iconGear", bsicons.Gear)
	cfg.RegisterIconFunc("iconBoxArrowRight", bsicons.BoxArrowRight)
	cfg.RegisterIconFunc("iconBoxArrowInRight", bsicons.BoxArrowInRight)
	return nil
}
func (a *App) TranslationFS() fs.FS { return translationFS }

// TemplateFS returns the embedded HTML template files.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return sub
}

// FuncMap returns template functions for the layout and home page.
func (a *App) FuncMap() template.FuncMap {
	return template.FuncMap{
		"alertClass": func(level messages.Level) string {
			if level == messages.Error {
				return "danger"
			}
			return string(level)
		},
	}
}

func (a *App) NavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{Label: "Home", URL: "/", Icon: bsicons.House(), Position: 1},
	}
}

func (a *App) Routes(r chi.Router) {
	r.Get("/", burrow.Handle(home))
}

// Logo returns a static brand logo HTML for auth pages.
func Logo() template.HTML {
	return `<h1 class="display-5 fw-bold">Burrow</h1>`
}

func home(w http.ResponseWriter, r *http.Request) error {
	return burrow.Render(w, r, http.StatusOK, "pages/home", map[string]any{"Title": "Home"})
}
