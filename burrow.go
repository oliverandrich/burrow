// Package burrow is a Go web framework built on chi, Bun/SQLite, and html/template.
// It provides a modular architecture where features are packaged as "apps" that
// plug into a shared server.
//
// # Getting Started
//
// Create a server, register apps, and run:
//
//	srv := burrow.NewServer(
//	    session.New(),
//	    csrf.New(),
//	    myapp.New(),
//	)
//	srv.SetLayout(myLayout)
//
//	app := &cli.Command{
//	    Name:   "mysite",
//	    Flags:  srv.Flags(nil),
//	    Action: srv.Run,
//	}
//	_ = app.Run(context.Background(), os.Args)
//
// NewServer sorts apps by declared dependencies automatically. The boot
// sequence runs migrations, calls Register on each app, configures them
// from CLI/ENV/TOML flags, and starts the HTTP server with graceful shutdown.
//
// # Handler Functions
//
// Burrow handlers return an error instead of silently swallowing failures:
//
//	func listItems(w http.ResponseWriter, r *http.Request) error {
//	    items, err := fetchItems(r.Context())
//	    if err != nil {
//	        return err // logged and rendered as 500
//	    }
//	    return burrow.JSON(w, http.StatusOK, items)
//	}
//
// Wrap them with [Handle] to get a standard http.HandlerFunc:
//
//	r.Get("/items", burrow.Handle(listItems))
//
// Return an [HTTPError] to control the status code and message:
//
//	return burrow.NewHTTPError(http.StatusNotFound, "item not found")
//
// # Response Helpers
//
// [JSON], [Text], and [HTML] write responses with correct Content-Type headers.
// [Render] writes pre-rendered template.HTML (useful for HTMX fragments).
// [RenderTemplate] executes a named template and wraps it in the layout for
// full-page requests, or returns the fragment alone for HTMX requests.
//
// # Request Binding and Validation
//
// [Bind] parses a request body (JSON, multipart, or form-encoded) into a struct
// and validates it using "validate" struct tags. On validation failure it returns
// a [*ValidationError] containing per-field errors:
//
//	type CreateItem struct {
//	    Name  string `form:"name"  validate:"required"`
//	    Email string `form:"email" validate:"required,email"`
//	}
//
//	func create(w http.ResponseWriter, r *http.Request) error {
//	    var input CreateItem
//	    if err := burrow.Bind(r, &input); err != nil {
//	        var ve *burrow.ValidationError
//	        if errors.As(err, &ve) {
//	            return burrow.JSON(w, 422, ve.Errors)
//	        }
//	        return err
//	    }
//	    // input is valid
//	}
//
// [Validate] can be called standalone on any struct.
//
// # App Interface
//
// Every app implements [App] (Name + Register). Apps gain additional capabilities
// by implementing optional interfaces:
//
//   - [Migratable] — embedded SQL migration files
//   - [HasRoutes] — HTTP route registration on a chi.Router
//   - [HasMiddleware] — global middleware
//   - [HasNavItems] — main navigation entries
//   - [HasTemplates] — .html template files parsed into the global template set
//   - [HasFuncMap] — static template functions
//   - [HasRequestFuncMap] — per-request template functions
//   - [Configurable] — CLI/ENV/TOML flags and configuration
//   - [HasCLICommands] — CLI subcommands
//   - [Seedable] — database seeding
//   - [HasAdmin] — admin panel routes and navigation
//   - [HasStaticFiles] — embedded static file assets
//   - [HasTranslations] — i18n translation files
//   - [HasDependencies] — declared app dependencies for ordering
//   - [HasShutdown] — graceful shutdown hooks
//
// # Templates
//
// Apps contribute .html files via [HasTemplates]. All templates are parsed into a
// single global [html/template.Template] at boot time. Templates use
// {{ define "appname/templatename" }} blocks for namespacing. Apps can add
// template functions via [HasFuncMap] (static) and [HasRequestFuncMap]
// (per-request, e.g. for CSRF tokens or the current user).
//
// # Pagination
//
// [ParsePageRequest] extracts limit and page from the query string.
// Use [ApplyOffset] + [OffsetResult] for offset-based pagination.
// [PageResponse] wraps items and pagination metadata for JSON APIs.
//
// # Contrib Apps
//
// The contrib/ directory provides reusable apps:
//
//   - auth — WebAuthn passkey authentication with recovery codes
//   - authmail — pluggable email rendering with SMTP backend
//   - session — cookie-based sessions (gorilla/sessions)
//   - csrf — CSRF protection (gorilla/csrf)
//   - i18n — locale detection and translations (go-i18n)
//   - admin — admin panel with generic CRUD via ModelAdmin
//   - bootstrap — Bootstrap 5 CSS/JS with dark mode
//   - bsicons — Bootstrap Icons as inline SVG template functions
//   - alpine — Alpine.js asset serving via staticfiles
//   - htmx — HTMX asset serving and request/response helpers
//   - jobs — SQLite-backed in-process job queue with retry
//   - sse — Server-Sent Events with in-memory pub/sub broker
//   - uploads — pluggable file upload storage
//   - messages — flash messages via session storage
//   - ratelimit — per-client token bucket rate limiting
//   - healthcheck — liveness and readiness probes
//   - staticfiles — static file serving with content-hashed URLs
package burrow

import (
	"context"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"
	"github.com/urfave/cli/v3"
)

// App is the required interface that all apps must implement.
// An app has a unique name and a Register method that receives
// the shared configuration needed to wire into the framework.
type App interface {
	Name() string
	Register(cfg *AppConfig) error
}

// IconFunc is the function signature for icon template functions.
// It matches the signature used by bsicons and other icon packages.
type IconFunc = func(...string) template.HTML

// AppConfig is passed to each app's Register method, providing
// access to shared framework resources.
type AppConfig struct {
	DB         *bun.DB
	Registry   *Registry
	Config     *Config
	WithLocale func(ctx context.Context, lang string) context.Context
	iconFuncs  map[string]IconFunc
}

// RegisterIconFunc registers an icon function for use in templates.
// Duplicate registrations of the same name are silently ignored,
// allowing multiple apps to depend on the same icon.
func (cfg *AppConfig) RegisterIconFunc(name string, fn IconFunc) {
	if cfg.iconFuncs == nil {
		cfg.iconFuncs = make(map[string]IconFunc)
	}
	if _, exists := cfg.iconFuncs[name]; !exists {
		cfg.iconFuncs[name] = fn
	}
}

// IconFuncs returns all registered icon functions.
func (cfg *AppConfig) IconFuncs() map[string]IconFunc {
	return cfg.iconFuncs
}

// NavItem represents a navigation entry contributed by an app.
type NavItem struct { //nolint:govet // fieldalignment: readability over optimization
	Label     string
	LabelKey  string // i18n message ID; translated at render time, falls back to Label
	URL       string
	Icon      template.HTML
	Position  int
	AuthOnly  bool
	AdminOnly bool
}

// NavLink is a template-ready navigation item with pre-computed active state.
// It is produced by the navLinks template function from the registered NavItems,
// filtered by the current user's authentication/authorization state.
type NavLink struct {
	Label    string
	URL      string
	Icon     template.HTML
	IsActive bool
}

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
// The configSource parameter enables TOML file sourcing; it may be nil
// when only ENV/CLI sources are used.
type Configurable interface {
	Flags(configSource func(key string) cli.ValueSource) []cli.Flag
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

// HasRequestFuncMap is implemented by apps that provide request-scoped
// template functions (e.g., CSRF tokens, current user, translations).
// These are added per request via middleware using template.Clone().
type HasRequestFuncMap interface {
	RequestFuncMap(r *http.Request) template.FuncMap
}
