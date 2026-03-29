package burrow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/urfave/cli/v3"
)

// Registry holds registered apps in insertion order and provides methods
// to collect capabilities (routes, middleware, flags, etc.) from all apps.
// Application code typically does not interact with Registry directly —
// [Server] manages it internally. Contrib app authors may use
// [Registry.Get] to look up sibling apps during Configure.
type Registry struct {
	index map[string]App
	apps  []App
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		index: make(map[string]App),
	}
}

// Add registers an app. It panics if an app with the same name
// has already been registered or if a declared dependency is missing
// (programming errors caught at startup).
func (r *Registry) Add(app App) {
	name := app.Name()
	if _, exists := r.index[name]; exists {
		panic(fmt.Sprintf("burrow: duplicate app name %q", name))
	}

	if dep, ok := app.(HasDependencies); ok {
		for _, required := range dep.Dependencies() {
			if _, exists := r.index[required]; !exists {
				panic(fmt.Sprintf("burrow: app %q requires %q to be registered first", name, required))
			}
		}
	}

	r.apps = append(r.apps, app)
	r.index[name] = app

	var caps []string
	if _, ok := app.(Migratable); ok {
		caps = append(caps, "migrations")
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
	if _, ok := app.(Seedable); ok {
		caps = append(caps, "seed")
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
	slog.Debug("app registered", "name", name, "capabilities", caps)
}

// Get returns the app with the given name, or false if not found.
func (r *Registry) Get(name string) (App, bool) {
	app, ok := r.index[name]
	return app, ok
}

// Apps returns all registered apps in the order they were added.
func (r *Registry) Apps() []App {
	result := make([]App, len(r.apps))
	copy(result, r.apps)
	return result
}

// ConfigureAll calls Configure on each Configurable app, passing the given
// AppConfig with a nil *cli.Command. This is a test convenience; the real
// boot sequence uses [Registry.Configure].
func (r *Registry) ConfigureAll(cfg *AppConfig) error {
	for _, app := range r.apps {
		if provider, ok := app.(Configurable); ok {
			if err := provider.Configure(cfg, nil); err != nil {
				return fmt.Errorf("configure app %q: %w", app.Name(), err)
			}
		}
	}
	return nil
}

// AllNavItems collects NavItems from all HasNavItems apps
// and returns them sorted by Position (stable sort preserves
// insertion order for equal positions).
func (r *Registry) AllNavItems() []NavItem {
	var items []NavItem
	for _, app := range r.apps {
		if provider, ok := app.(HasNavItems); ok {
			items = append(items, provider.NavItems()...)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Position < items[j].Position
	})
	return items
}

// RegisterMiddleware applies middleware from all HasMiddleware apps
// to the chi router, in app registration order.
func (r *Registry) RegisterMiddleware(router chi.Router) {
	for _, app := range r.apps {
		if provider, ok := app.(HasMiddleware); ok {
			for _, mw := range provider.Middleware() {
				router.Use(mw)
			}
		}
	}
}

// AllFlags collects CLI flags from all HasFlags apps.
// Pass configSource to enable TOML file sourcing (or nil for ENV-only).
func (r *Registry) AllFlags(configSource func(key string) cli.ValueSource) []cli.Flag {
	var flags []cli.Flag
	for _, app := range r.apps {
		if provider, ok := app.(HasFlags); ok {
			flags = append(flags, provider.Flags(configSource)...)
		}
	}
	return flags
}

// Configure calls Configure on each Configurable app, then calls
// PostConfigure on each PostConfigurable app. This two-phase approach
// guarantees that all apps are fully configured before any PostConfigure
// runs, which is important for apps that need to interact with other
// apps' state (e.g., the jobs app discovering HasJobs handlers).
func (r *Registry) Configure(cfg *AppConfig, cmd *cli.Command) error {
	for _, app := range r.apps {
		if provider, ok := app.(Configurable); ok {
			if err := provider.Configure(cfg, cmd); err != nil {
				return fmt.Errorf("configure app %q: %w", app.Name(), err)
			}
		}
	}
	for _, app := range r.apps {
		if provider, ok := app.(PostConfigurable); ok {
			if err := provider.PostConfigure(cfg, cmd); err != nil {
				return fmt.Errorf("post-configure app %q: %w", app.Name(), err)
			}
		}
	}
	return nil
}

// AllCLICommands collects CLI subcommands from all HasCLICommands apps.
func (r *Registry) AllCLICommands() []*cli.Command {
	var cmds []*cli.Command
	for _, app := range r.apps {
		if provider, ok := app.(HasCLICommands); ok {
			cmds = append(cmds, provider.CLICommands()...)
		}
	}
	return cmds
}

// AllAdminNavItems collects AdminNavItems from all HasAdmin apps
// and returns them sorted by Position (stable sort preserves
// insertion order for equal positions).
func (r *Registry) AllAdminNavItems() []NavItem {
	var items []NavItem
	for _, app := range r.apps {
		if provider, ok := app.(HasAdmin); ok {
			items = append(items, provider.AdminNavItems()...)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Position < items[j].Position
	})
	return items
}

// RegisterRoutes calls Routes on each HasRoutes app,
// allowing apps to register their HTTP handlers.
func (r *Registry) RegisterRoutes(router chi.Router) {
	for _, app := range r.apps {
		if provider, ok := app.(HasRoutes); ok {
			provider.Routes(router)
		}
	}
}

// Shutdown calls Shutdown on each HasShutdown app in reverse
// registration order. Errors are collected but do not prevent
// other apps from shutting down.
func (r *Registry) Shutdown(ctx context.Context) error {
	var errs []error
	for i := len(r.apps) - 1; i >= 0; i-- {
		if provider, ok := r.apps[i].(HasShutdown); ok {
			if err := provider.Shutdown(ctx); err != nil {
				slog.Error("app shutdown error", "app", r.apps[i].Name(), "error", err)
				errs = append(errs, fmt.Errorf("shutdown app %q: %w", r.apps[i].Name(), err))
			}
		}
	}
	return errors.Join(errs...)
}

// Seed calls Seed on each Seedable app in order.
// It stops and returns on the first error.
func (r *Registry) Seed(ctx context.Context) error {
	for _, app := range r.apps {
		if provider, ok := app.(Seedable); ok {
			if err := provider.Seed(ctx); err != nil {
				return fmt.Errorf("seed app %q: %w", app.Name(), err)
			}
		}
	}
	return nil
}
