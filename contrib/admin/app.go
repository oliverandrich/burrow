package admin

import (
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/admin/templates"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"github.com/go-chi/chi/v5"
)

// App implements the admin coordinator contrib app.
type App struct {
	registry *burrow.Registry
	layout   burrow.LayoutFunc
}

// New creates a new admin app. The optional layout wraps admin pages.
func New(layout burrow.LayoutFunc) *App {
	return &App{layout: layout}
}

// Layout returns a batteries-included admin layout using Bootstrap 5
// and htmx. It serves as a ready-to-use admin layout that reads static file
// URLs and nav items from the request context.
func Layout() burrow.LayoutFunc {
	return templates.Layout
}

func (a *App) Name() string { return "admin" }

func (a *App) Dependencies() []string { return []string{"auth"} }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.registry = cfg.Registry
	return nil
}

// Middleware returns middleware that injects admin nav items into the
// request context. The admin layout is injected inside the /admin route
// group instead, so that it only applies to admin pages.
func (a *App) Middleware() []func(http.Handler) http.Handler {
	items := a.registry.AllAdminNavItems()
	return []func(http.Handler) http.Handler{
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := WithNavItems(r.Context(), items)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		},
	}
}

// indexPage renders the admin dashboard page.
func (a *App) indexPage(w http.ResponseWriter, r *http.Request) error {
	content := templates.AdminIndex(NavItems(r.Context()))
	layout := burrow.Layout(r.Context())
	if layout == nil {
		return burrow.Render(w, r, http.StatusOK, content)
	}
	return burrow.Render(w, r, http.StatusOK, layout("Admin", content))
}

// Routes creates the /admin group with auth middleware and delegates
// to all HasAdmin apps.
func (a *App) Routes(r chi.Router) {
	if a.registry == nil {
		return
	}

	r.Route("/admin", func(r chi.Router) {
		r.Use(auth.RequireAuth(), auth.RequireAdmin())

		// Inject admin layout inside the /admin group so it overrides the
		// app layout only for admin pages.
		if a.layout != nil {
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx := burrow.WithLayout(r.Context(), a.layout)
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
		}

		r.Get("/", burrow.Handle(a.indexPage))

		for _, app := range a.registry.Apps() {
			if provider, ok := app.(burrow.HasAdmin); ok {
				provider.AdminRoutes(r)
			}
		}
	})
}
