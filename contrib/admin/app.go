package admin

import (
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"github.com/go-chi/chi/v5"
)

// App implements the admin coordinator contrib app.
type App struct {
	registry *burrow.Registry
}

// New creates a new admin app.
func New() *App {
	return &App{}
}

func (a *App) Name() string { return "admin" }

func (a *App) Dependencies() []string { return []string{"auth"} }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.registry = cfg.Registry
	return nil
}

// Middleware returns middleware that injects admin nav items into the context.
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

// Routes creates the /admin group with auth middleware and delegates
// to all HasAdmin apps.
func (a *App) Routes(r chi.Router) {
	if a.registry == nil {
		return
	}

	r.Route("/admin", func(r chi.Router) {
		r.Use(auth.RequireAuth(), auth.RequireAdmin())

		for _, app := range a.registry.Apps() {
			if provider, ok := app.(burrow.HasAdmin); ok {
				provider.AdminRoutes(r)
			}
		}
	})
}
