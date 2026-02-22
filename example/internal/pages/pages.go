// Package pages provides the example app's static pages (homepage)
// and request-path middleware for active nav link highlighting.
package pages

import (
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/example/internal/layout"
	"codeberg.org/oliverandrich/burrow/example/internal/pages/templates"
	"github.com/go-chi/chi/v5"
)

// App implements the pages app.
type App struct{}

// New creates a new pages app.
func New() *App { return &App{} }

func (a *App) Name() string                       { return "pages" }
func (a *App) Register(_ *burrow.AppConfig) error { return nil }

func (a *App) NavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{Label: "Home", URL: "/", Icon: "bi bi-house", Position: 1},
	}
}

// Middleware injects the request path into context for nav link highlighting.
func (a *App) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{layout.Middleware()}
}

func (a *App) Routes(r chi.Router) {
	r.Get("/", burrow.Handle(home))
}

func home(w http.ResponseWriter, r *http.Request) error {
	layout := burrow.Layout(r.Context())
	if layout != nil {
		return burrow.Render(w, r, http.StatusOK, layout("Home", templates.HomePage()))
	}
	return burrow.Render(w, r, http.StatusOK, templates.HomePage())
}
