// Package pages provides the layout and homepage for the tutorial application.
package pages

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"codeberg.org/oliverandrich/burrow/contrib/messages"
	"github.com/go-chi/chi/v5"
)

//go:embed templates
var templateFS embed.FS

type App struct{}

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
			return burrow.HTML(w, code, string(content))
		}

		layoutData := map[string]any{
			"Content":  content,
			"NavItems": burrow.NavItems(r.Context()),
			"Messages": messages.Get(r.Context()),
			"User":     auth.UserFromContext(r.Context()),
		}
		if title, ok := data["Title"]; ok {
			layoutData["Title"] = title
		}

		rendered, err := exec(r, "app/layout", layoutData)
		if err != nil {
			return err
		}
		return burrow.HTML(w, code, string(rendered))
	}
}
