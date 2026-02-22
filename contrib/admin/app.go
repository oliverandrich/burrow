package admin

import (
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"github.com/go-chi/chi/v5"
)

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

// New creates a new admin app. The layout wraps admin pages, and dashboard
// renders the index page. Both are optional (pass nil for either).
func New(layout burrow.LayoutFunc, dashboard DashboardRenderer) *App {
	return &App{layout: layout, dashboard: dashboard}
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
