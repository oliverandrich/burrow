// Package app defines the foundation types and capability interfaces every
// burrow app implements. Apps satisfy [App] (required) and any number of the
// optional capability interfaces (HasRoutes, HasMiddleware, HasMigrations, …)
// to opt into framework features.
//
// The root burrow package re-exports the most-used names from this package as
// type aliases so that downstream code can keep writing burrow.App,
// burrow.AppConfig, burrow.HasRoutes and so on. Less-frequent names are
// reached by importing this package directly.
package app

import (
	"context"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow/registry"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"
	"github.com/oliverandrich/den/migrate"
	"github.com/urfave/cli/v3"
)

// App is the required interface that all apps must implement.
// An app has a unique name used for identification in the registry,
// migrations, and logging.
type App = registry.App

// AppConfig is passed to each app's Configure method, providing
// access to shared framework resources.
type AppConfig struct {
	DB         *den.DB
	Registry   *registry.Registry
	Config     *Config
	WithLocale func(ctx context.Context, lang string) context.Context
}

// NavItem represents a navigation entry contributed by an app.
//
// Label doubles as the i18n message ID: it is passed through [i18n.T] at
// render time, so contribute translations keyed by the English Label. When
// no translation matches, the raw Label is rendered.
//
// Icon is the name of a template define (e.g. "auth/icon_people") rendered by
// the layout via {{ template .Icon . }}. Each contrib keeps its icons in
// templates/icons.html as {{ define "<app>/icon_<name>" }} blocks.
type NavItem struct { //nolint:govet // fieldalignment: readability over optimization
	Label     string
	URL       string
	Icon      string
	Position  int
	AuthOnly  bool
	StaffOnly bool
	AdminOnly bool
}

// NavLink is a template-ready navigation item with pre-computed active state.
// It is produced by the navLinks template function from the registered NavItems,
// filtered by the current user's authentication/authorization state.
//
// Icon is a template name; see [NavItem].
type NavLink struct {
	Label    string
	URL      string
	Icon     string
	IsActive bool
}

// HasDocuments is implemented by apps that register Den document types.
// The returned slice should contain zero-value pointers, e.g. &User{}, &Job{}.
// [document.Document] is Den's sealed marker interface — only types that
// embed [document.Base] satisfy it, so non-document types fail at compile
// time. Den's Register() creates tables and indexes automatically from the
// struct tags.
type HasDocuments interface {
	Documents() []document.Document
}

// HasMiddleware is implemented by apps that contribute HTTP middleware.
type HasMiddleware interface {
	Middleware() []func(http.Handler) http.Handler
}

// HasNavItems is implemented by apps that contribute navigation items.
type HasNavItems interface {
	NavItems() []NavItem
}

// HasFlags is implemented by apps that define CLI flags.
// The configSource parameter enables TOML file sourcing; it may be nil
// when only ENV/CLI sources are used.
type HasFlags interface {
	Flags(configSource func(key string) cli.ValueSource) []cli.Flag
}

// Configurable is implemented by apps that need to read their configuration
// and perform setup (create repositories, register icons, wire handlers).
// Configure receives the shared [AppConfig] and the parsed CLI command.
type Configurable interface {
	Configure(cfg *AppConfig, cmd *cli.Command) error
}

// HasCLICommands is implemented by apps that contribute subcommands.
type HasCLICommands interface {
	CLICommands() []*cli.Command
}

// HasRoutes is implemented by apps that register HTTP routes.
type HasRoutes interface {
	Routes(r chi.Router)
}

// HasMigrations is implemented by apps that ship versioned, run-once
// migrations on top of the auto-discovered document schema. The server
// applies them automatically at boot via Den's migrate package — each
// migration runs exactly once across processes, tracked in the
// _den_migrations collection. Versions are namespaced by app name so two
// contribs can both ship "001_initial" without colliding.
type HasMigrations interface {
	Migrations() []NamedMigration
}

// NamedMigration pairs a version label with a Den migrate.Migration. The
// Version is the lexicographic ordering key inside an app; cross-app order
// follows the registry's dependency-resolved app order.
//
// Migration.Forward is required. Migration.Backward is optional — omitting
// it locks the migration as forward-only; rollback via migrate.Down /
// migrate.DownOne returns an error for a migration without a Backward.
type NamedMigration struct {
	Version   string
	Migration migrate.Migration
}

// AdminAuth provides authentication and authorization middleware for the
// admin panel. The admin app discovers an AdminAuth provider from the
// registry during Configure and uses its middleware to protect /admin routes.
// contrib/auth implements this interface.
//
// RequireAuth gates "logged in or not"; RequireStaff gates "may enter the
// admin shell" (used by the admin coordinator for the /admin/ frame);
// RequireAdmin gates "full admin privileges" (used per-route by apps).
// Roles form a hierarchy: admin implies staff implies authenticated.
type AdminAuth interface {
	RequireAuth() func(http.Handler) http.Handler
	RequireStaff() func(http.Handler) http.Handler
	RequireAdmin() func(http.Handler) http.Handler
}

// HasAdmin is implemented by apps that contribute admin panel routes
// and navigation items. AdminRoutes receives a chi router already
// prefixed with /admin and protected by auth middleware.
type HasAdmin interface {
	AdminRoutes(r chi.Router)
	AdminNavItems() []NavItem
}

// HasStaticFiles is implemented by apps that contribute static file
// assets. The returned prefix namespaces the files under the static
// URL path (e.g., prefix "admin" serves files at /static/admin/...).
type HasStaticFiles interface {
	StaticFS() (prefix string, fsys fs.FS)
}

// HasTranslations is implemented by apps that contribute translation
// files. The returned fs.FS must contain a "translations/" directory
// with TOML files (e.g., "translations/active.en.toml").
type HasTranslations interface {
	TranslationFS() fs.FS
}

// PostConfigurable is implemented by apps that need a second configuration
// pass after all [Configurable] apps have been configured. This is useful
// when an app needs to interact with other apps' state that is only available
// after Configure() has run (e.g., the jobs app discovering HasJobs handlers).
// PostConfigure is called once, after all Configure() calls have completed.
type PostConfigurable interface {
	PostConfigure(cfg *AppConfig, cmd *cli.Command) error
}

// HasShutdown is implemented by apps that need to perform cleanup
// during graceful shutdown (e.g., stopping background goroutines,
// flushing buffers). Called in reverse registration order before
// the HTTP server stops.
type HasShutdown interface {
	Shutdown(ctx context.Context) error
}

// ReadinessChecker is implemented by apps that contribute to the
// readiness probe. ReadinessCheck returns nil when the app is ready
// to serve traffic, or an error describing what is not ready.
type ReadinessChecker interface {
	ReadinessCheck(ctx context.Context) error
}

// HasTemplates is implemented by apps that provide HTML template files.
// The returned fs.FS should contain .html files with {{ define "appname/..." }}
// blocks. Templates are parsed once at boot time into the global template set.
type HasTemplates interface {
	TemplateFS() fs.FS
}

// HasFuncMap is implemented by apps that provide static template functions.
// These are added once at boot time and available in all templates.
type HasFuncMap interface {
	FuncMap() template.FuncMap
}

// HasRequestFuncMap is implemented by apps that provide context-scoped
// template functions (e.g., CSRF tokens, current user, translations).
// These are added per request via middleware using template.Clone().
// The context carries all request-scoped values needed by the functions;
// this enables template rendering outside HTTP handlers (background jobs,
// SSE broadcasts, CLI commands).
type HasRequestFuncMap interface {
	RequestFuncMap(ctx context.Context) template.FuncMap
}
