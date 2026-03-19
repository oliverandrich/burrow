package burrow

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
	"github.com/oliverandrich/burrow/i18n"
	"github.com/uptrace/bun"
	"github.com/urfave/cli/v3"
)

// Server is the main framework entry point that orchestrates the full
// application lifecycle. Typical usage:
//
//  1. Create with [NewServer] (registers apps, sorts by dependencies)
//  2. Configure the layout with [Server.SetLayout]
//  3. Collect CLI flags with [Server.Flags]
//  4. Start with [Server.Run] (opens DB, migrates, bootstraps, serves HTTP)
type Server struct { //nolint:govet // fieldalignment: readability over optimization
	registry                *Registry
	layout                  string
	templates               *template.Template
	requestFuncMapProviders []func(r *http.Request) template.FuncMap
	i18nBundle              *i18n.Bundle
	appCfg                  *AppConfig
}

// NewServer creates a Server and registers the given apps.
// Apps are automatically sorted so that dependencies are registered
// before the apps that need them. The relative order of independent
// apps is preserved. NewServer panics if a dependency is missing
// from the input or if there is a dependency cycle.
func NewServer(apps ...App) *Server {
	sorted := sortApps(apps)
	reg := NewRegistry()
	for _, app := range sorted {
		reg.Add(app)
	}
	return &Server{registry: reg}
}

// sortApps performs a stable topological sort of apps based on their
// declared dependencies (HasDependencies interface). Apps without
// dependencies keep their original relative order. Panics if a
// required dependency is not in the input or if a cycle is detected.
func sortApps(apps []App) []App {
	// Build index and in-degree map.
	byName := make(map[string]int, len(apps)) // name → original index
	for i, app := range apps {
		byName[app.Name()] = i
	}

	inDegree := make([]int, len(apps))
	dependents := make([][]int, len(apps)) // dependents[i] = apps that depend on apps[i]

	for i, app := range apps {
		dep, ok := app.(HasDependencies)
		if !ok {
			continue
		}
		for _, required := range dep.Dependencies() {
			j, exists := byName[required]
			if !exists {
				panic(fmt.Sprintf("burrow: app %q requires %q, but it was not provided", app.Name(), required))
			}
			inDegree[i]++
			dependents[j] = append(dependents[j], i)
		}
	}

	// Kahn's algorithm with a queue seeded in original order (stable).
	queue := make([]int, 0, len(apps))
	for i := range apps {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}

	sorted := make([]App, 0, len(apps))
	for len(queue) > 0 {
		idx := queue[0]
		queue = queue[1:]
		sorted = append(sorted, apps[idx])

		for _, depIdx := range dependents[idx] {
			inDegree[depIdx]--
			if inDegree[depIdx] == 0 {
				queue = append(queue, depIdx)
			}
		}
	}

	// Warn if the order changed so the developer can fix their registration order.
	reordered := false
	for i, app := range sorted {
		if app.Name() != apps[i].Name() {
			reordered = true
			break
		}
	}
	if reordered {
		original := make([]string, len(apps))
		for i, app := range apps {
			original[i] = app.Name()
		}
		result := make([]string, len(sorted))
		for i, app := range sorted {
			result[i] = app.Name()
		}
		slog.Warn("app registration order was adjusted to satisfy dependencies",
			"original", original, "resolved", result)
	}

	if len(sorted) != len(apps) {
		// Find apps involved in the cycle for a useful error message.
		var cycled []string
		for i, deg := range inDegree {
			if deg > 0 {
				cycled = append(cycled, apps[i].Name())
			}
		}
		panic(fmt.Sprintf("burrow: dependency cycle detected among apps: %v", cycled))
	}

	return sorted
}

// SetLayout configures the layout template name used for all pages.
func (s *Server) SetLayout(name string) {
	s.layout = name
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
	flags = append(flags, s.registry.AllFlags(configSource)...)
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

	// Create i18n bundle from core config (before bootstrap so apps get WithLocale).
	langs := strings.Split(cfg.I18n.SupportedLanguages, ",")
	bundle, err := i18n.NewBundle(cfg.I18n.DefaultLanguage, langs)
	if err != nil {
		return fmt.Errorf("create i18n bundle: %w", err)
	}
	s.i18nBundle = bundle

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

	// Load translations from all HasTranslations apps (after Register).
	for _, app := range s.registry.Apps() {
		if p, ok := app.(HasTranslations); ok {
			if err := s.i18nBundle.AddTranslations(p.TranslationFS()); err != nil {
				return fmt.Errorf("load translations from %q: %w", app.Name(), err)
			}
		}
	}

	if err := s.registry.Configure(cmd); err != nil {
		return err
	}

	// Register core request func map providers.
	s.requestFuncMapProviders = append(s.requestFuncMapProviders, s.i18nBundle.RequestFuncMap)
	s.requestFuncMapProviders = append(s.requestFuncMapProviders, coreRequestFuncMap)

	if err := s.buildTemplates(); err != nil {
		return fmt.Errorf("build templates: %w", err)
	}

	r := chi.NewRouter()
	r.Use(httplog.RequestLogger(slog.Default(), &httplog.Options{
		Level:         slog.LevelInfo,
		RecoverPanics: true,
	}))
	r.Use(chimw.RequestID)
	r.Use(chimw.Compress(5))
	r.Use(chimw.RequestSize(int64(cfg.Server.MaxBodySize) * 1024 * 1024))
	r.Use(s.i18nBundle.LocaleMiddleware())
	navItems := s.registry.AllNavItems()
	r.Use(navItemsMiddleware(navItems))
	if s.layout != "" {
		r.Use(layoutMiddleware(s.layout))
	}
	if s.templates != nil {
		r.Use(s.templateMiddleware())
	}
	s.registry.RegisterMiddleware(r)
	s.registry.RegisterRoutes(r)

	r.NotFound(Handle(func(w http.ResponseWriter, r *http.Request) error {
		return NewHTTPError(http.StatusNotFound, "page not found")
	}))
	r.MethodNotAllowed(Handle(func(w http.ResponseWriter, r *http.Request) error {
		return NewHTTPError(http.StatusMethodNotAllowed, "method not allowed")
	}))

	return startServer(ctx, r, cfg, s.registry)
}

// bootstrap runs migrations, registers all apps, and seeds the database.
func (s *Server) bootstrap(ctx context.Context, db *bun.DB, cfg *Config) error {
	if err := s.registry.RunMigrations(ctx, db); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	s.appCfg = &AppConfig{
		DB:         db,
		Registry:   s.registry,
		Config:     cfg,
		WithLocale: s.i18nBundle.WithLocale,
	}
	for _, app := range s.registry.Apps() {
		if err := app.Register(s.appCfg); err != nil {
			return fmt.Errorf("register app %q: %w", app.Name(), err)
		}
	}

	return s.registry.Seed(ctx)
}

func layoutMiddleware(name string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithLayout(r.Context(), name)
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
