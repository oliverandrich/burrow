// Package pages provides the example app's static pages (homepage),
// layout rendering, and request-path middleware for active nav link highlighting.
package pages

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"maps"
	"net/http"
	"strings"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"codeberg.org/oliverandrich/burrow/contrib/bsicons"
	"codeberg.org/oliverandrich/burrow/contrib/messages"
	"github.com/go-chi/chi/v5"
)

//go:embed translations
var translationFS embed.FS

//go:embed templates
var templateFS embed.FS

// ctxKeyRequestPath is used to pass the request path into the template context.
type ctxKeyRequestPath struct{}

// App implements the pages app.
type App struct{}

// New creates a new pages app.
func New() *App { return &App{} }

func (a *App) Name() string                       { return "pages" }
func (a *App) Register(_ *burrow.AppConfig) error { return nil }
func (a *App) TranslationFS() fs.FS               { return translationFS }

// TemplateFS returns the embedded HTML template files.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return sub
}

// FuncMap returns template functions for the layout and home page.
func (a *App) FuncMap() template.FuncMap {
	return template.FuncMap{
		"iconHouse":           func(class ...string) template.HTML { return bsicons.House(class...) },
		"iconKey":             func(class ...string) template.HTML { return bsicons.Key(class...) },
		"iconPuzzle":          func(class ...string) template.HTML { return bsicons.Puzzle(class...) },
		"iconLightning":       func(class ...string) template.HTML { return bsicons.Lightning(class...) },
		"iconGear":            func(class ...string) template.HTML { return bsicons.Gear(class...) },
		"iconBoxArrowRight":   func(class ...string) template.HTML { return bsicons.BoxArrowRight(class...) },
		"iconBoxArrowInRight": func(class ...string) template.HTML { return bsicons.BoxArrowInRight(class...) },
		"alertClass": func(level string) string {
			if level == string(messages.Error) {
				return "danger"
			}
			return level
		},
	}
}

func (a *App) NavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{Label: "Home", URL: "/", Icon: bsicons.House(), Position: 1},
	}
}

// Middleware injects the request path into context for nav link highlighting.
func (a *App) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), ctxKeyRequestPath{}, r.URL.Path)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		},
	}
}

func (a *App) Routes(r chi.Router) {
	r.Get("/", burrow.Handle(home))
}

// navItemData holds a pre-computed nav item for template rendering.
type navItemData struct {
	Label     string
	URL       string
	Icon      template.HTML
	LinkClass string
}

// Layout returns a LayoutFunc that wraps page content in the app layout.
func Layout() burrow.LayoutFunc {
	return func(w http.ResponseWriter, r *http.Request, code int, content template.HTML, data map[string]any) error {
		exec := burrow.TemplateExecutorFromContext(r.Context())
		if exec == nil {
			return burrow.HTML(w, code, string(content))
		}

		ctx := r.Context()
		currentPath, _ := ctx.Value(ctxKeyRequestPath{}).(string)

		// Pre-compute nav items with CSS classes.
		visible := visibleNavItems(ctx)
		navItems := make([]navItemData, len(visible))
		for i, item := range visible {
			navItems[i] = navItemData{
				Label:     item.Label,
				URL:       item.URL,
				Icon:      item.Icon,
				LinkClass: navLinkClass(currentPath, item.URL),
			}
		}

		layoutData := make(map[string]any, len(data)+4)
		maps.Copy(layoutData, data)
		layoutData["Content"] = content
		layoutData["NavItems"] = navItems
		layoutData["User"] = auth.UserFromContext(ctx)
		layoutData["Messages"] = messages.Get(ctx)
		if _, ok := layoutData["Title"]; !ok {
			layoutData["Title"] = ""
		}

		html, err := exec(r, "app/layout", layoutData)
		if err != nil {
			return err
		}
		return burrow.HTML(w, code, string(html))
	}
}

// Logo returns a static brand logo HTML for auth pages.
func Logo() template.HTML {
	return `<h1 class="display-5 fw-bold">Burrow</h1>`
}

// visibleNavItems returns nav items the current user should see.
func visibleNavItems(ctx context.Context) []burrow.NavItem {
	user := auth.UserFromContext(ctx)
	var visible []burrow.NavItem
	for _, item := range burrow.NavItems(ctx) {
		if item.AuthOnly && user == nil {
			continue
		}
		if item.AdminOnly && (user == nil || !user.IsAdmin()) {
			continue
		}
		visible = append(visible, item)
	}
	return visible
}

// navLinkClass returns CSS classes for a nav link, marking it active
// when it matches the current path.
func navLinkClass(currentPath, itemURL string) string {
	if currentPath == "" {
		return "nav-link"
	}
	if itemURL == "/" {
		if currentPath == "/" {
			return "nav-link active"
		}
		return "nav-link"
	}
	if strings.HasPrefix(currentPath, itemURL) {
		return "nav-link active"
	}
	return "nav-link"
}

func home(w http.ResponseWriter, r *http.Request) error {
	exec := burrow.TemplateExecutorFromContext(r.Context())
	if exec == nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "no template executor")
	}

	content, err := exec(r, "pages/home", nil)
	if err != nil {
		return err
	}

	layout := burrow.Layout(r.Context())
	if layout != nil {
		return layout(w, r, http.StatusOK, content, map[string]any{"Title": "Home"})
	}
	return burrow.Render(w, r, http.StatusOK, content)
}
