// Package pages provides the layout and homepage for the tutorial application.
package pages

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"github.com/go-chi/chi/v5"
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

func (a *App) FuncMap() template.FuncMap { return nil }

func (a *App) NavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{Label: "Home", URL: "/", Position: 0},
	}
}

func (a *App) Routes(r chi.Router) {
	r.Get("/", burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return burrow.RenderTemplate(w, r, http.StatusOK, "pages/home", map[string]any{
			"Title": "Welcome to Polls",
		})
	}))
}

// Layout returns a LayoutFunc that wraps content in the app layout template.
func Layout() burrow.LayoutFunc {
	return func(w http.ResponseWriter, r *http.Request, code int, content template.HTML, data map[string]any) error {
		exec := burrow.TemplateExecutorFromContext(r.Context())
		if exec == nil {
			w.WriteHeader(code)
			_, err := w.Write([]byte(content))
			return err
		}

		layoutData := map[string]any{
			"Content":  content,
			"NavItems": burrow.NavItems(r.Context()),
		}
		if title, ok := data["Title"]; ok {
			layoutData["Title"] = title
		}

		rendered, err := exec(r, "app/layout", layoutData)
		if err != nil {
			return err
		}
		w.WriteHeader(code)
		_, err = w.Write([]byte(rendered))
		return err
	}
}
