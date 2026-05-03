// Package admin provides the admin panel coordinator as a burrow contrib app.
// It handles layout, navigation, dashboard rendering, and acts as the mount
// point for admin views contributed by apps implementing HasAdmin.
package admin

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/bsicons"
	"github.com/urfave/cli/v3"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed translations
var translationFS embed.FS

//go:embed static
var staticFS embed.FS

// DashboardRenderer renders the admin dashboard page.
type DashboardRenderer interface {
	DashboardPage(w http.ResponseWriter, r *http.Request) error
}

// App implements the admin coordinator contrib app.
type App struct {
	dashboard      DashboardRenderer
	registry       *burrow.Registry
	authMiddleware burrow.AdminAuth
	layout         string
}

// Option configures the admin app.
type Option func(*App)

// WithLayout sets the layout template name for admin pages.
func WithLayout(layout string) Option {
	return func(a *App) { a.layout = layout }
}

// WithDashboardRenderer sets the dashboard page renderer.
func WithDashboardRenderer(r DashboardRenderer) Option {
	return func(a *App) { a.dashboard = r }
}

// New creates a new admin app with the given options.
// By default, the built-in HTML layout and dashboard renderer are used.
// Use WithLayout() and WithDashboardRenderer() to override.
func New(opts ...Option) *App {
	a := &App{
		layout:    DefaultLayout(),
		dashboard: DefaultDashboardRenderer(),
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func (a *App) Name() string { return "admin" }

// Dependencies declares contribs the admin app's templates assume are present:
// staticfiles serves admin.css, htmx powers the boosted nav, mucss provides
// the css / theme switcher / pagination templates referenced from admin/layout,
// and messages provides the flash-message template function used at the top
// of every admin page.
func (a *App) Dependencies() []string {
	return []string{"staticfiles", "htmx", "mucss", "messages"}
}

func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
	a.registry = cfg.Registry
	cfg.RegisterIconFunc("iconPersonCircle", bsicons.PersonCircle)

	// Discover the AdminAuth provider from the registry.
	for _, app := range cfg.Registry.Apps() {
		if aa, ok := app.(burrow.AdminAuth); ok {
			if a.authMiddleware != nil {
				first, _ := a.authMiddleware.(burrow.App) //nolint:errcheck // AdminAuth providers are always Apps
				return fmt.Errorf("admin: multiple AdminAuth providers found (%s and %s)", first.Name(), app.Name())
			}
			a.authMiddleware = aa
		}
	}
	if a.authMiddleware == nil {
		return fmt.Errorf("admin: no AdminAuth provider found (register an auth app before admin)")
	}
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

// TranslationFS returns the embedded translation files (admin UI labels).
func (a *App) TranslationFS() fs.FS { return translationFS }

// TemplateFS returns the embedded HTML template files.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return sub
}

// StaticFS returns the embedded admin static assets under the "admin" prefix.
func (a *App) StaticFS() (string, fs.FS) {
	sub, _ := fs.Sub(staticFS, "static")
	return "admin", sub
}

// RequestFuncMap returns request-scoped template functions for the admin
// dashboard.
func (a *App) RequestFuncMap(ctx context.Context) template.FuncMap {
	return template.FuncMap{
		"adminDashboard": func() []DashboardGroup {
			return PrepareDashboard(ctx, NavGroups(ctx))
		},
	}
}

// Routes creates the /admin group with auth middleware and delegates
// to all HasAdmin apps.
func (a *App) Routes(r chi.Router) {
	if a.registry == nil {
		return
	}

	r.Route("/admin", func(r chi.Router) {
		r.Use(a.authMiddleware.RequireAuth(), a.authMiddleware.RequireAdmin())

		groups := a.buildNavGroups()

		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				if a.layout != "" {
					ctx = burrow.WithLayout(ctx, a.layout)
				}
				ctx = WithNavGroups(ctx, groups)
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
