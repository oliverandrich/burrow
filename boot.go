package burrow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow/registry"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/migrate"
	"github.com/urfave/cli/v3"
)

// registerApp adds an app to the registry and logs the implemented
// capability interfaces at debug level. NewServer is the canonical
// caller; tests that exercise the registry directly use [registry.Add].
func registerApp(reg *Registry, app App) {
	registry.Add(reg, app)
	logCapabilities(app)
}

// logCapabilities emits a debug log entry naming every optional
// capability interface the app implements. The list is informational
// only; it has no effect on behaviour.
func logCapabilities(app App) {
	var caps []string
	if _, ok := app.(HasDocuments); ok {
		caps = append(caps, "documents")
	}
	if _, ok := app.(HasRoutes); ok {
		caps = append(caps, "routes")
	}
	if _, ok := app.(HasMiddleware); ok {
		caps = append(caps, "middleware")
	}
	if _, ok := app.(HasNavItems); ok {
		caps = append(caps, "nav")
	}
	if _, ok := app.(HasFlags); ok {
		caps = append(caps, "flags")
	}
	if _, ok := app.(Configurable); ok {
		caps = append(caps, "config")
	}
	if _, ok := app.(HasCLICommands); ok {
		caps = append(caps, "commands")
	}
	if _, ok := app.(HasMigrations); ok {
		caps = append(caps, "migrations")
	}
	if _, ok := app.(HasAdmin); ok {
		caps = append(caps, "admin")
	}
	if _, ok := app.(HasStaticFiles); ok {
		caps = append(caps, "staticfiles")
	}
	if _, ok := app.(HasTranslations); ok {
		caps = append(caps, "translations")
	}
	if _, ok := app.(HasDependencies); ok {
		caps = append(caps, "dependencies")
	}
	if _, ok := app.(HasShutdown); ok {
		caps = append(caps, "shutdown")
	}
	if _, ok := app.(ReadinessChecker); ok {
		caps = append(caps, "readiness")
	}
	if _, ok := app.(HasTemplates); ok {
		caps = append(caps, "templates")
	}
	if _, ok := app.(HasFuncMap); ok {
		caps = append(caps, "funcmap")
	}
	if _, ok := app.(HasRequestFuncMap); ok {
		caps = append(caps, "requestfuncmap")
	}
	if _, ok := app.(HasJobs); ok {
		caps = append(caps, "jobs")
	}
	if _, ok := app.(PostConfigurable); ok {
		caps = append(caps, "postconfigure")
	}
	if _, ok := app.(Startable); ok {
		caps = append(caps, "startable")
	}
	slog.Debug("app registered", "name", app.Name(), "capabilities", caps)
}

// configureAll calls Configure on each Configurable app with a nil
// *cli.Command. It is a test convenience; the real boot sequence uses
// [configure] (two-phase Configure + PostConfigure with the parsed command).
func configureAll(reg *Registry, cfg *AppConfig) error {
	for _, app := range registry.Apps(reg) {
		if provider, ok := app.(Configurable); ok {
			if err := provider.Configure(cfg, nil); err != nil {
				return fmt.Errorf("configure app %q: %w", app.Name(), err)
			}
		}
	}
	return nil
}

// configure runs the two-phase configure sequence: every Configurable
// app's Configure first, then every PostConfigurable's PostConfigure.
// The two phases ensure all apps are fully configured before any
// post-configure runs (important for apps that need other apps' state,
// such as jobs discovering HasJobs handlers).
func configure(reg *Registry, cfg *AppConfig, cmd *cli.Command) error {
	apps := registry.Apps(reg)
	for _, app := range apps {
		if provider, ok := app.(Configurable); ok {
			if err := provider.Configure(cfg, cmd); err != nil {
				return fmt.Errorf("configure app %q: %w", app.Name(), err)
			}
		}
	}
	for _, app := range apps {
		if provider, ok := app.(PostConfigurable); ok {
			if err := provider.PostConfigure(cfg, cmd); err != nil {
				return fmt.Errorf("post-configure app %q: %w", app.Name(), err)
			}
		}
	}
	return nil
}

// registerMiddleware applies middleware from all HasMiddleware apps to
// the chi router in registration order.
func registerMiddleware(reg *Registry, router chi.Router) {
	for _, app := range registry.Apps(reg) {
		if provider, ok := app.(HasMiddleware); ok {
			for _, mw := range provider.Middleware() {
				router.Use(mw)
			}
		}
	}
}

// registerRoutes calls Routes on each HasRoutes app, letting apps wire
// their HTTP handlers onto the router.
func registerRoutes(reg *Registry, router chi.Router) {
	for _, app := range registry.Apps(reg) {
		if provider, ok := app.(HasRoutes); ok {
			provider.Routes(router)
		}
	}
}

// runMigrations builds a [migrate.Registry] from every HasMigrations app
// and applies all pending migrations. Each migration runs exactly once
// across processes — Den tracks applied versions in the _den_migrations
// collection. Versions are namespaced by app name so two contribs can
// both ship "001_initial" without colliding.
func runMigrations(ctx context.Context, reg *Registry, db *den.DB) error {
	mreg := migrate.NewRegistry()
	for _, app := range registry.Apps(reg) {
		provider, ok := app.(HasMigrations)
		if !ok {
			continue
		}
		for _, nm := range provider.Migrations() {
			mreg.Register(app.Name()+"/"+nm.Version, nm.Migration)
		}
	}
	return mreg.Up(ctx, db)
}

// registerDocuments registers document types from all HasDocuments apps
// with the Den database. Den creates tables and indexes automatically
// from the struct tags.
func registerDocuments(ctx context.Context, reg *Registry, db *den.DB) error {
	for _, app := range registry.Apps(reg) {
		hd, ok := app.(HasDocuments)
		if !ok {
			continue
		}
		if err := den.Register(ctx, db, hd.Documents()...); err != nil {
			return fmt.Errorf("register documents for %q: %w", app.Name(), err)
		}
	}
	return nil
}

// allFlags collects CLI flags from all HasFlags apps. Pass configSource
// to enable TOML file sourcing, or nil for ENV-only.
func allFlags(reg *Registry, configSource func(key string) cli.ValueSource) []cli.Flag {
	var flags []cli.Flag
	for _, app := range registry.Apps(reg) {
		if provider, ok := app.(HasFlags); ok {
			flags = append(flags, provider.Flags(configSource)...)
		}
	}
	return flags
}

// allNavItems collects NavItems from all HasNavItems apps, sorted by
// Position (stable sort preserves insertion order for equal positions).
func allNavItems(reg *Registry) []NavItem {
	var items []NavItem
	for _, app := range registry.Apps(reg) {
		if provider, ok := app.(HasNavItems); ok {
			items = append(items, provider.NavItems()...)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Position < items[j].Position
	})
	return items
}

// allAdminNavItems collects AdminNavItems from all HasAdmin apps,
// sorted by Position.
func allAdminNavItems(reg *Registry) []NavItem {
	var items []NavItem
	for _, app := range registry.Apps(reg) {
		if provider, ok := app.(HasAdmin); ok {
			items = append(items, provider.AdminNavItems()...)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Position < items[j].Position
	})
	return items
}

// allCLICommands collects CLI subcommands from all HasCLICommands apps.
func allCLICommands(reg *Registry) []*cli.Command {
	var cmds []*cli.Command
	for _, app := range registry.Apps(reg) {
		if provider, ok := app.(HasCLICommands); ok {
			cmds = append(cmds, provider.CLICommands()...)
		}
	}
	return cmds
}

// shutdownApps calls Shutdown on each HasShutdown app in reverse
// registration order. Errors are collected but do not prevent other
// apps from shutting down.
func shutdownApps(ctx context.Context, reg *Registry) error {
	var errs []error
	apps := registry.Apps(reg)
	for _, v := range slices.Backward(apps) {
		if provider, ok := v.(HasShutdown); ok {
			if err := provider.Shutdown(ctx); err != nil {
				slog.Error("app shutdown error", "app", v.Name(), "error", err)
				errs = append(errs, fmt.Errorf("shutdown app %q: %w", v.Name(), err))
			}
		}
	}
	return errors.Join(errs...)
}
