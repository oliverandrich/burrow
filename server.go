package burrow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

// checkDBDir verifies that the parent directory of a file-based DSN exists.
// In-memory databases (":memory:" or empty DSN) are skipped.
func checkDBDir(dsn string) error {
	if dsn == "" || dsn == ":memory:" || strings.HasPrefix(dsn, "file::memory") {
		return nil
	}

	// Strip query parameters from file: URIs.
	path := dsn
	if after, ok := strings.CutPrefix(path, "file:"); ok {
		path = after
		if i := strings.IndexByte(path, '?'); i >= 0 {
			path = path[:i]
		}
	}

	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("database directory %q does not exist; create it with: mkdir -p %s", dir, dir)
	}
	return nil
}

func openDB(dsn string) (*bun.DB, error) {
	if err := checkDBDir(dsn); err != nil {
		return nil, err
	}

	dsn = withTxLock(dsn)

	sqldb, err := sql.Open(sqliteshim.ShimName, dsn)
	if err != nil {
		return nil, err
	}

	sqldb.SetMaxOpenConns(10)
	sqldb.SetMaxIdleConns(5)
	sqldb.SetConnMaxLifetime(time.Hour)

	db := bun.NewDB(sqldb, sqlitedialect.New())

	pragmas := []struct {
		sql    string
		errMsg string
	}{
		{"PRAGMA journal_mode=WAL", "set WAL mode"},
		{"PRAGMA synchronous=NORMAL", "set synchronous"},
		{"PRAGMA foreign_keys=ON", "enable foreign keys"},
		{"PRAGMA busy_timeout=5000", "set busy timeout"},
		{"PRAGMA temp_store=MEMORY", "set temp store"},
		{"PRAGMA mmap_size=134217728", "set mmap size"},
		{"PRAGMA journal_size_limit=27103364", "set journal size limit"},
		{"PRAGMA cache_size=2000", "set cache size"},
	}

	for _, p := range pragmas {
		if _, err := db.Exec(p.sql); err != nil {
			return nil, fmt.Errorf("%s: %w", p.errMsg, err)
		}
	}

	return db, nil
}

// withTxLock ensures the DSN uses IMMEDIATE transaction mode.
// This prevents transactions from failing immediately when the database is
// locked and instead waits up to busy_timeout before returning an error.
func withTxLock(dsn string) string {
	if strings.Contains(dsn, "_txlock=") {
		return dsn
	}

	switch {
	case dsn == ":memory:" || strings.HasPrefix(dsn, "file::memory"):
		return dsn
	case strings.HasPrefix(dsn, "file:"):
		if strings.Contains(dsn, "?") {
			return dsn + "&_txlock=immediate"
		}
		return dsn + "?_txlock=immediate"
	default:
		return "file:" + dsn + "?_txlock=immediate"
	}
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
