package burrow

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/urfave/cli/v3"
)

// Server is the main framework entry point. It holds the Registry
// of apps and orchestrates the boot sequence.
type Server struct {
	registry                *Registry
	layout                  LayoutFunc
	templates               *template.Template
	requestFuncMapProviders []func(r *http.Request) template.FuncMap
}

// NewServer creates a Server and registers the given apps.
func NewServer(apps ...App) *Server {
	reg := NewRegistry()
	for _, app := range apps {
		reg.Add(app)
	}
	return &Server{registry: reg}
}

// SetLayout configures the app layout function.
func (s *Server) SetLayout(fn LayoutFunc) {
	s.layout = fn
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

	if err := cfg.ValidateTLS(cmd); err != nil {
		return err
	}

	if cfg.Server.BaseURL == "" {
		cfg.Server.BaseURL = cfg.ResolveBaseURL()
	}

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

	if err := s.buildTemplates(); err != nil {
		return fmt.Errorf("build templates: %w", err)
	}

	r := chi.NewRouter()
	setupCoreMiddleware(r, cfg)
	navItems := s.registry.AllNavItems()
	r.Use(navItemsMiddleware(navItems))
	if s.layout != nil {
		r.Use(layoutMiddleware(s.layout))
	}
	if s.templates != nil {
		r.Use(s.templateMiddleware())
	}
	s.registry.RegisterMiddleware(r)
	s.registry.RegisterRoutes(r)

	return startServer(ctx, r, cfg, s.registry)
}

// bootstrap runs migrations, registers all apps, and seeds the database.
func (s *Server) bootstrap(ctx context.Context, db *bun.DB, cfg *Config) error {
	if err := s.registry.RunMigrations(ctx, db); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	appCfg := &AppConfig{
		DB:       db,
		Registry: s.registry,
		Config:   cfg,
	}
	for _, app := range s.registry.Apps() {
		if err := app.Register(appCfg); err != nil {
			return fmt.Errorf("register app %q: %w", app.Name(), err)
		}
	}

	return s.registry.Seed(ctx)
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

func layoutMiddleware(fn LayoutFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithLayout(r.Context(), fn)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func navItemsMiddleware(items []NavItem) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithNavItems(r.Context(), items)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func setupCoreMiddleware(r chi.Router, cfg *Config) {
	r.Use(httplog.RequestLogger(slog.Default(), &httplog.Options{
		Level:         slog.LevelInfo,
		RecoverPanics: true,
	}))
	r.Use(chimw.RequestID)
	r.Use(chimw.Compress(5))
	r.Use(chimw.RequestSize(int64(cfg.Server.MaxBodySize) * 1024 * 1024))
}

// shutdownServers performs graceful shutdown of the HTTP servers and registry.
func shutdownServers(cfg *Config, registry *Registry, server *http.Server, httpServer *http.Server) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := registry.Shutdown(shutdownCtx); err != nil {
		slog.Error("app shutdown errors", "error", err)
	}

	if httpServer != nil {
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("http redirect server shutdown error", "error", err)
		}
	}

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
		return err
	}

	slog.Info("server stopped")
	return nil
}
