// Package healthcheck provides a minimal health check app for burrow.
package healthcheck

import (
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"
)

// New creates a new healthcheck app.
func New() *App { return &App{} }

// App implements the burrow.App and burrow.HasRoutes interfaces.
// It registers a /healthz endpoint that returns the server and database status.
type App struct {
	db *bun.DB
}

func (a *App) Name() string { return "healthcheck" }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.db = cfg.DB
	return nil
}

func (a *App) Routes(r chi.Router) {
	r.Get("/healthz", burrow.Handle(a.health))
}

func (a *App) health(w http.ResponseWriter, r *http.Request) error {
	dbStatus := "ok"
	if err := a.db.PingContext(r.Context()); err != nil {
		dbStatus = err.Error()
	}

	status := http.StatusOK
	if dbStatus != "ok" {
		status = http.StatusServiceUnavailable
	}

	return burrow.JSON(w, status, map[string]string{
		"status":   "ok",
		"database": dbStatus,
	})
}
