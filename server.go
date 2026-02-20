package burrow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/urfave/cli/v3"
)

// Server is the main framework entry point. It holds the Registry
// of apps and orchestrates the boot sequence.
type Server struct {
	registry *Registry
	layouts  Layouts
}

// NewServer creates a Server and registers the given apps.
func NewServer(apps ...App) *Server {
	reg := NewRegistry()
	for _, app := range apps {
		reg.Add(app)
	}
	return &Server{registry: reg}
}

// SetLayouts configures the app and admin layout functions.
func (s *Server) SetLayouts(l Layouts) {
	s.layouts = l
}

// Registry returns the server's app registry.
func (s *Server) Registry() *Registry {
	return s.registry
}

// Flags returns all CLI flags: core framework flags merged with
// flags from all Configurable apps. Pass a configSource to enable
// TOML file sourcing (or nil for ENV-only).
func (s *Server) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
	flags := CoreFlags(configSource)
	flags = append(flags, s.registry.AllFlags()...)
	return flags
}

// Run is a cli.ActionFunc that boots and starts the server.
// It opens the database, runs migrations, bootstraps apps,
// configures apps, and starts the HTTP server with graceful shutdown.
func (s *Server) Run(ctx context.Context, cmd *cli.Command) error {
	cfg := NewConfig(cmd)

	if cfg.Server.BaseURL == "" {
		cfg.Server.BaseURL = cfg.ResolveBaseURL()
	}

	setupLogger(cfg.Log.Level, cfg.Log.Format)

	slog.Info("starting server",
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
		"base_url", cfg.Server.BaseURL,
	)

	db, err := openDB(cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Error("failed to close database", "error", closeErr)
		}
	}()

	if err := s.bootstrap(ctx, db, cfg); err != nil {
		return err
	}

	if err := s.registry.Configure(cmd); err != nil {
		return err
	}

	e := echo.New()
	setupCoreMiddleware(e, cfg)
	navItems := s.registry.AllNavItems()
	e.Use(navItemsMiddleware(navItems))
	s.registry.RegisterMiddleware(e)
	s.registry.RegisterRoutes(e)

	return startServer(ctx, e, cfg)
}

// bootstrap runs migrations and registers all apps.
func (s *Server) bootstrap(ctx context.Context, db *bun.DB, cfg *Config) error {
	if err := s.registry.RunMigrations(ctx, db); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	appCfg := &AppConfig{
		DB:       db,
		Registry: s.registry,
		Config:   cfg,
		Layouts:  s.layouts,
	}
	for _, app := range s.registry.Apps() {
		if err := app.Register(appCfg); err != nil {
			return fmt.Errorf("register app %q: %w", app.Name(), err)
		}
	}
	return nil
}

func openDB(dsn string) (*bun.DB, error) {
	sqldb, err := sql.Open(sqliteshim.ShimName, dsn)
	if err != nil {
		return nil, err
	}

	sqldb.SetMaxOpenConns(10)
	sqldb.SetMaxIdleConns(5)
	sqldb.SetConnMaxLifetime(time.Hour)

	db := bun.NewDB(sqldb, sqlitedialect.New())

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		return nil, fmt.Errorf("set synchronous: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	return db, nil
}

func navItemsMiddleware(items []NavItem) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := WithNavItems(c.Request().Context(), items)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

func setupCoreMiddleware(e *echo.Echo, cfg *Config) {
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.Gzip())
	e.Use(middleware.BodyLimit(int64(cfg.Server.MaxBodySize) * 1024 * 1024))
}

func setupLogger(level, format string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}

	slog.SetDefault(slog.New(handler))
}

func startServer(ctx context.Context, e *echo.Echo, cfg *Config) error {
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           e,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errChan := make(chan error, 1)
	go func() {
		slog.Info("server listening", "addr", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		slog.Info("context cancelled, shutting down")
	case <-quit:
		slog.Info("signal received, shutting down")
	case err := <-errChan:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
		return err
	}

	slog.Info("server stopped")
	return nil
}
