package admin

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"codeberg.org/oliverandrich/burrow/contrib/bsicons"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed translations
var translationFS embed.FS

// DashboardRenderer renders the admin dashboard page.
type DashboardRenderer interface {
	DashboardPage(w http.ResponseWriter, r *http.Request) error
}

// App implements the admin coordinator contrib app.
type App struct {
	registry  *burrow.Registry
	layout    burrow.LayoutFunc
	dashboard DashboardRenderer
}

// Option configures the admin app.
type Option func(*App)

// WithLayout sets the layout for admin pages.
func WithLayout(fn burrow.LayoutFunc) Option {
	return func(a *App) { a.layout = fn }
}

// WithDashboardRenderer sets the dashboard page renderer.
func WithDashboardRenderer(r DashboardRenderer) Option {
	return func(a *App) { a.dashboard = r }
}

// New creates a new admin app with the given options.
func New(opts ...Option) *App {
	a := &App{}
	for _, o := range opts {
		o(a)
	}
	return a
}

func (a *App) Name() string { return "admin" }

func (a *App) Dependencies() []string { return []string{"auth"} }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.registry = cfg.Registry
	return nil
}

// indexPage renders the admin dashboard page.
func (a *App) indexPage(w http.ResponseWriter, r *http.Request) error {
	if a.dashboard != nil {
		return a.dashboard.DashboardPage(w, r)
	}
	return burrow.Text(w, http.StatusOK, "admin dashboard")
}

// buildNavGroups collects nav groups from all HasAdmin apps.
func (a *App) buildNavGroups() []NavGroup {
	var groups []NavGroup
	for _, app := range a.registry.Apps() {
		if provider, ok := app.(burrow.HasAdmin); ok {
			items := provider.AdminNavItems()
			if len(items) > 0 {
				groups = append(groups, NavGroup{
					AppName: app.Name(),
					Items:   items,
				})
			}
		}
	}
	return groups
}

// TranslationFS returns the embedded translation files (modeladmin UI labels).
func (a *App) TranslationFS() fs.FS { return translationFS }

// TemplateFS returns the embedded HTML template files.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return sub
}

// FuncMap returns static template functions for admin icons.
func (a *App) FuncMap() template.FuncMap {
	return template.FuncMap{
		"iconGearFill":     func(class ...string) template.HTML { return bsicons.GearFill(class...) },
		"iconChevronRight": func(class ...string) template.HTML { return bsicons.ChevronRight(class...) },
		"iconPersonCircle": func(class ...string) template.HTML { return bsicons.PersonCircle(class...) },
	}
}

// Routes creates the /admin group with auth middleware and delegates
// to all HasAdmin apps.
func (a *App) Routes(r chi.Router) {
	if a.registry == nil {
		return
	}

	r.Route("/admin", func(r chi.Router) {
		r.Use(auth.RequireAuth(), auth.RequireAdmin())

		groups := a.buildNavGroups()

		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				if a.layout != nil {
					ctx = burrow.WithLayout(ctx, a.layout)
				}
				ctx = WithNavGroups(ctx, groups)
				ctx = WithRequestPath(ctx, r.URL.Path)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})

		r.Get("/", burrow.Handle(a.indexPage))

		for _, app := range a.registry.Apps() {
			if provider, ok := app.(burrow.HasAdmin); ok {
				provider.AdminRoutes(r)
			}
		}
	})
}
