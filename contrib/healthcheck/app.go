// Package healthcheck provides liveness and readiness probes for burrow.
//
// It registers two endpoints:
//   - GET /healthz/live — always returns 200 (liveness probe)
//   - GET /healthz/ready — database ping + all ReadinessChecker apps (200/503)
package healthcheck

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/uptrace/bun"
)

// New creates a new healthcheck app.
func New() *App { return &App{} }

// App implements the burrow.App and burrow.HasRoutes interfaces.
// It registers liveness and readiness endpoints.
type App struct {
	db       *bun.DB
	registry *burrow.Registry
}

func (a *App) Name() string { return "healthcheck" }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.db = cfg.DB
	a.registry = cfg.Registry
	return nil
}

func (a *App) Routes(r chi.Router) {
	r.Get("/healthz/live", burrow.Handle(a.liveness))
	r.Get("/healthz/ready", burrow.Handle(a.readiness))
}

func (a *App) liveness(w http.ResponseWriter, _ *http.Request) error {
	return burrow.JSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (a *App) readiness(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	allOK := true

	dbStatus := "ok"
	if err := a.db.PingContext(ctx); err != nil {
		dbStatus = err.Error()
		allOK = false
	}

	checks := make(map[string]string)
	for _, app := range a.registry.Apps() {
		checker, ok := app.(burrow.ReadinessChecker)
		if !ok {
			continue
		}
		if err := checker.ReadinessCheck(ctx); err != nil {
			checks[app.Name()] = err.Error()
			allOK = false
		} else {
			checks[app.Name()] = "ok"
		}
	}

	overallStatus := "ok"
	httpStatus := http.StatusOK
	if !allOK {
		overallStatus = "unavailable"
		httpStatus = http.StatusServiceUnavailable
	}

	return burrow.JSON(w, httpStatus, map[string]any{
		"status":   overallStatus,
		"database": dbStatus,
		"checks":   checks,
	})
}
