package burrow

import (
	"context"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/urfave/cli/v3"
)

// Migratable is implemented by apps that provide database migrations.
type Migratable interface {
	MigrationFS() fs.FS
}

// HasMiddleware is implemented by apps that contribute HTTP middleware.
type HasMiddleware interface {
	Middleware() []func(http.Handler) http.Handler
}

// HasNavItems is implemented by apps that contribute navigation items.
type HasNavItems interface {
	NavItems() []NavItem
}

// Configurable is implemented by apps that define CLI flags
// and need to read their configuration from the CLI command.
type Configurable interface {
	Flags() []cli.Flag
	Configure(cmd *cli.Command) error
}

// HasCLICommands is implemented by apps that contribute subcommands.
type HasCLICommands interface {
	CLICommands() []*cli.Command
}

// HasRoutes is implemented by apps that register HTTP routes.
type HasRoutes interface {
	Routes(r chi.Router)
}

// Seedable is implemented by apps that can seed the database
// with initial data.
type Seedable interface {
	Seed(ctx context.Context) error
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

// HasDependencies is implemented by apps that require other apps
// to be registered first. Dependencies() returns the names of
// required apps; registration panics if any are missing.
type HasDependencies interface {
	Dependencies() []string
}

// HasShutdown is implemented by apps that need to perform cleanup
// during graceful shutdown (e.g., stopping background goroutines,
// flushing buffers). Called in reverse registration order before
// the HTTP server stops.
type HasShutdown interface {
	Shutdown(ctx context.Context) error
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

// HasRequestFuncMap is implemented by apps that provide request-scoped
// template functions (e.g., CSRF tokens, current user, translations).
// These are added per request via middleware using template.Clone().
type HasRequestFuncMap interface {
	RequestFuncMap(r *http.Request) template.FuncMap
}
