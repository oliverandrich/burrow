// Package healthcheck provides a minimal health check app for core.
package healthcheck

import (
	"net/http"

	"codeberg.org/oliverandrich/go-webapp-template/core"
	"github.com/labstack/echo/v5"
	"github.com/uptrace/bun"
)

// App implements the core.App and core.HasRoutes interfaces.
// It registers a /healthz endpoint that returns the server and database status.
type App struct {
	db *bun.DB
}

func (a *App) Name() string { return "healthcheck" }

func (a *App) Register(cfg *core.AppConfig) error {
	a.db = cfg.DB
	return nil
}

func (a *App) Routes(e *echo.Echo) {
	e.GET("/healthz", a.health)
}

func (a *App) health(c *echo.Context) error {
	dbStatus := "ok"
	if err := a.db.PingContext(c.Request().Context()); err != nil {
		dbStatus = err.Error()
	}

	status := http.StatusOK
	if dbStatus != "ok" {
		status = http.StatusServiceUnavailable
	}

	return c.JSON(status, map[string]string{
		"status":   "ok",
		"database": dbStatus,
	})
}
