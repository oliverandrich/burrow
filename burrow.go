// Package burrow is a Go web framework built on chi, Den, and html/template.
// It provides a modular architecture where features are packaged as "apps" that
// plug into a shared server.
//
// The root burrow package re-exports the most-used names from the sub-packages
// as type aliases and thin wrapper functions, so downstream code can keep
// writing burrow.App, burrow.AppConfig, burrow.NewServer, burrow.Handle and so
// on. Less-frequent names live in their proper sub-package:
//
//   - [github.com/oliverandrich/burrow/app] — App interface, capability
//     interfaces (HasRoutes, HasMiddleware, …), AppConfig, NavItem, NavLink,
//     Config and TLS/Server/Database/Storage sub-configs, context helpers
//   - [github.com/oliverandrich/burrow/registry] — the app registry with
//     typed lookup (Get[T], MustGet[T], …)
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
// sequence runs migrations, configures each app from CLI/ENV/TOML flags,
// and starts the HTTP server with graceful shutdown.
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
// # App Interface
//
// Every app implements [App] (Name only). Apps gain additional capabilities
// by implementing optional interfaces such as [HasRoutes], [HasMiddleware],
// [HasMigrations], [Configurable], [HasShutdown], and others — all defined
// in the [github.com/oliverandrich/burrow/app] package.
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
//   - htmx — HTMX asset serving and request/response helpers
//   - jobs — Den-backed in-process job queue with retry (SQLite + Postgres)
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
	"net/http"

	"github.com/oliverandrich/burrow/app"
	"github.com/oliverandrich/burrow/pagination"
	"github.com/oliverandrich/burrow/registry"
	"github.com/oliverandrich/burrow/server"
	"github.com/oliverandrich/burrow/tasks"
	"github.com/oliverandrich/burrow/web"
	"github.com/oliverandrich/den"
	"github.com/urfave/cli/v3"
)

// App is the required interface that every burrow app implements. Alias for
// [app.App].
type App = app.App

// AppConfig is passed to each app's Configure method, providing access to
// shared framework resources. Alias for [app.AppConfig].
type AppConfig = app.AppConfig

// Registry stores the apps that make up a Server. Alias for
// [registry.Registry] — operate on it with the free functions in package
// registry: [registry.New], [registry.Add], [registry.Get], etc.
type Registry = registry.Registry

// NavItem represents a navigation entry contributed by an app. Alias for
// [app.NavItem].
type NavItem = app.NavItem

// NavLink is a template-ready navigation item with pre-computed active state.
// Alias for [app.NavLink].
type NavLink = app.NavLink

// Config holds core framework configuration. Alias for [app.Config].
type Config = app.Config

// ServerConfig holds HTTP server settings. Alias for [app.ServerConfig].
type ServerConfig = app.ServerConfig

// DatabaseConfig holds database settings. Alias for [app.DatabaseConfig].
type DatabaseConfig = app.DatabaseConfig

// StorageConfig holds file-storage settings. Alias for [app.StorageConfig].
type StorageConfig = app.StorageConfig

// TLSConfig holds TLS settings. Alias for [app.TLSConfig].
type TLSConfig = app.TLSConfig

// TemplateExecutor executes a named template with the given data. Alias for
// [app.TemplateExecutor].
type TemplateExecutor = app.TemplateExecutor

// AuthChecker carries auth-state closures in the request context. Alias for
// [app.AuthChecker].
type AuthChecker = app.AuthChecker

// HasDocuments is implemented by apps that register Den document types.
// Alias for [app.HasDocuments].
type HasDocuments = app.HasDocuments

// HasMiddleware is implemented by apps that contribute HTTP middleware.
// Alias for [app.HasMiddleware].
type HasMiddleware = app.HasMiddleware

// HasNavItems is implemented by apps that contribute navigation items.
// Alias for [app.HasNavItems].
type HasNavItems = app.HasNavItems

// HasFlags is implemented by apps that define CLI flags. Alias for
// [app.HasFlags].
type HasFlags = app.HasFlags

// Configurable is implemented by apps that need to read configuration and
// perform setup. Alias for [app.Configurable].
type Configurable = app.Configurable

// HasCLICommands is implemented by apps that contribute CLI subcommands.
// Alias for [app.HasCLICommands].
type HasCLICommands = app.HasCLICommands

// HasRoutes is implemented by apps that register HTTP routes. Alias for
// [app.HasRoutes].
type HasRoutes = app.HasRoutes

// HasMigrations is implemented by apps that ship versioned migrations.
// Alias for [app.HasMigrations].
type HasMigrations = app.HasMigrations

// NamedMigration pairs a version label with a Den migration. Alias for
// [app.NamedMigration].
type NamedMigration = app.NamedMigration

// AdminAuth provides middleware for the admin panel. Alias for
// [app.AdminAuth].
type AdminAuth = app.AdminAuth

// HasAdmin is implemented by apps that contribute admin routes and nav items.
// Alias for [app.HasAdmin].
type HasAdmin = app.HasAdmin

// HasStaticFiles is implemented by apps that contribute static assets.
// Alias for [app.HasStaticFiles].
type HasStaticFiles = app.HasStaticFiles

// HasTranslations is implemented by apps that contribute translation files.
// Alias for [app.HasTranslations].
type HasTranslations = app.HasTranslations

// HasDependencies is implemented by apps that require other apps to be
// registered first. Alias for [registry.HasDependencies].
type HasDependencies = registry.HasDependencies

// PostConfigurable is implemented by apps that need a second configuration
// pass after Configure has run on every Configurable app. Alias for
// [app.PostConfigurable].
type PostConfigurable = app.PostConfigurable

// HasShutdown is implemented by apps that need cleanup during graceful
// shutdown. Alias for [app.HasShutdown].
type HasShutdown = app.HasShutdown

// ReadinessChecker is implemented by apps that contribute to the readiness
// probe. Alias for [app.ReadinessChecker].
type ReadinessChecker = app.ReadinessChecker

// HasTemplates is implemented by apps that provide HTML template files.
// Alias for [app.HasTemplates].
type HasTemplates = app.HasTemplates

// HasFuncMap is implemented by apps that provide static template functions.
// Alias for [app.HasFuncMap].
type HasFuncMap = app.HasFuncMap

// HasRequestFuncMap is implemented by apps that provide context-scoped
// template functions. Alias for [app.HasRequestFuncMap].
type HasRequestFuncMap = app.HasRequestFuncMap

// Server is the framework's main orchestrator. Alias for [server.Server].
// Construct with [NewServer]; configure layout via [server.Server.SetLayout];
// start with [server.Server.Run] inside a urfave/cli action.
type Server = server.Server

// Startable is implemented by apps that need to start background processes
// after the boot sequence completes. Alias for [server.Startable].
type Startable = server.Startable

// Queue provides job handler registration, enqueueing, and cancellation.
// Alias for [tasks.Queue].
type Queue = tasks.Queue

// Enqueuer provides job submission and cancellation. Alias for [tasks.Enqueuer].
type Enqueuer = tasks.Enqueuer

// JobConfig holds per-handler job configuration. Alias for [tasks.JobConfig].
type JobConfig = tasks.JobConfig

// JobOption configures job handler registration. Alias for [tasks.JobOption].
type JobOption = tasks.JobOption

// JobHandlerFunc is the signature for job handler functions. Alias for
// [tasks.JobHandlerFunc].
type JobHandlerFunc = tasks.JobHandlerFunc

// HasJobs is implemented by apps that register background job handlers.
// Alias for [tasks.HasJobs].
type HasJobs = tasks.HasJobs

// TaskDefinition is a generic typed task wrapper. Alias for
// [tasks.TaskDefinition].
type TaskDefinition[P any] = tasks.TaskDefinition[P]

// ResultTask is a generic typed task that captures a result. Alias for
// [tasks.ResultTask].
type ResultTask[P, R any] = tasks.ResultTask[P, R]

// ResultCapture holds the captured result of a job handler. Alias for
// [tasks.ResultCapture].
type ResultCapture = tasks.ResultCapture

// PageRequest captures requested limit and page from the query string. Alias
// for [pagination.PageRequest].
type PageRequest = pagination.PageRequest

// PageResult holds the computed pagination metadata. Alias for
// [pagination.PageResult].
type PageResult = pagination.PageResult

// PageResponse wraps items and pagination metadata for JSON APIs. Alias for
// [pagination.PageResponse].
type PageResponse[T any] = pagination.PageResponse[T]

// HandlerFunc is burrow's error-returning HTTP handler signature. Alias for
// [web.HandlerFunc].
type HandlerFunc = web.HandlerFunc

// HTTPError is the typed error returned from handlers to control status code
// and message. Alias for [web.HTTPError].
type HTTPError = web.HTTPError

// ValidationError carries per-field validation failures. Alias for
// [web.ValidationError].
type ValidationError = web.ValidationError

// FieldError is one entry in a [ValidationError]. Alias for [web.FieldError].
type FieldError = web.FieldError

// CacheControlImmutable is the value to set on Cache-Control headers for
// content-hashed static assets. Alias for [web.CacheControlImmutable].
const CacheControlImmutable = web.CacheControlImmutable

// ErrNoTemplateExecutor is returned by Render when no executor is in context.
// Alias for [web.ErrNoTemplateExecutor].
var ErrNoTemplateExecutor = web.ErrNoTemplateExecutor

// NewServer creates a Server and registers the given apps. Apps are
// auto-sorted to satisfy [HasDependencies] declarations. Wrapper around
// [server.New].
func NewServer(apps ...App) *Server { return server.New(apps...) }

// OpenDB opens a database from a URL-style DSN. Wrapper around
// [server.OpenDB].
func OpenDB(ctx context.Context, dsn string, opts ...den.Option) (*den.DB, error) {
	return server.OpenDB(ctx, dsn, opts...)
}

// NewConfig creates a Config from a parsed CLI command. Wrapper around
// [app.NewConfig].
func NewConfig(cmd *cli.Command) *Config { return app.NewConfig(cmd) }

// CoreFlags returns the CLI flags for core framework configuration. Wrapper
// around [app.CoreFlags].
func CoreFlags(configSource func(key string) cli.ValueSource) []cli.Flag {
	return app.CoreFlags(configSource)
}

// FlagSources builds a cli.ValueSourceChain from an environment variable and
// an optional TOML key. Wrapper around [app.FlagSources].
func FlagSources(configSource func(key string) cli.ValueSource, envVar, tomlKey string) cli.ValueSourceChain {
	return app.FlagSources(configSource, envVar, tomlKey)
}

// IsLocalhost reports whether the host string refers to a localhost address.
// Wrapper around [app.IsLocalhost].
func IsLocalhost(host string) bool { return app.IsLocalhost(host) }

// WithMaxRetries sets the maximum number of retries for a job type. Wrapper
// around [tasks.WithMaxRetries].
func WithMaxRetries(n int) JobOption { return tasks.WithMaxRetries(n) }

// WithPriority sets the default priority for a job type. Wrapper around
// [tasks.WithPriority].
func WithPriority(n int) JobOption { return tasks.WithPriority(n) }

// DefineTask wires up a generic typed task definition. Wrapper around
// [tasks.DefineTask].
func DefineTask[P any](name string, handler func(context.Context, P) error, opts ...JobOption) *TaskDefinition[P] {
	return tasks.DefineTask(name, handler, opts...)
}

// DefineResultTask wires up a generic typed task that captures a result.
// Wrapper around [tasks.DefineResultTask].
func DefineResultTask[P, R any](name string, handler func(context.Context, P) (R, error), opts ...JobOption) *ResultTask[P, R] {
	return tasks.DefineResultTask(name, handler, opts...)
}

// WithResultCapture stores a ResultCapture in the context. Wrapper around
// [tasks.WithResultCapture].
func WithResultCapture(ctx context.Context, rc *ResultCapture) context.Context {
	return tasks.WithResultCapture(ctx, rc)
}

// CaptureResult stores result data in the context-bound capture. Wrapper
// around [tasks.CaptureResult].
func CaptureResult(ctx context.Context, data []byte) { tasks.CaptureResult(ctx, data) }

// ParsePageRequest extracts limit and page from the request query string.
// Wrapper around [pagination.ParsePageRequest].
func ParsePageRequest(r *http.Request) PageRequest { return pagination.ParsePageRequest(r) }

// OffsetResult computes pagination metadata from a request and total count.
// Wrapper around [pagination.OffsetResult].
func OffsetResult(pr PageRequest, totalCount int) PageResult {
	return pagination.OffsetResult(pr, totalCount)
}

// PageURL builds a pagination URL preserving existing query parameters.
// Wrapper around [pagination.PageURL].
func PageURL(basePath, rawQuery string, page int) string {
	return pagination.PageURL(basePath, rawQuery, page)
}

// PageNumbers returns the truncated page list for a paginator UI. Wrapper
// around [pagination.PageNumbers].
func PageNumbers(current, total int) []int { return pagination.PageNumbers(current, total) }

// NewHTTPError constructs a typed HTTP error. Wrapper around
// [web.NewHTTPError].
func NewHTTPError(code int, message string) *HTTPError { return web.NewHTTPError(code, message) }

// Handle wraps a [HandlerFunc] into an http.HandlerFunc with centralized
// error handling. Wrapper around [web.Handle].
func Handle(fn HandlerFunc) http.HandlerFunc { return web.Handle(fn) }

// JSON writes a JSON response with the given status code. Wrapper around
// [web.JSON].
func JSON(w http.ResponseWriter, code int, v any) error { return web.JSON(w, code, v) }

// Text writes a plain-text response with the given status code. Wrapper
// around [web.Text].
func Text(w http.ResponseWriter, code int, s string) error { return web.Text(w, code, s) }

// HTML writes an HTML response with the given status code. Wrapper around
// [web.HTML].
func HTML(w http.ResponseWriter, code int, s string) error { return web.HTML(w, code, s) }

// Bind parses a request body (JSON, multipart, or form-encoded) into v and
// validates it. Wrapper around [web.Bind].
func Bind(r *http.Request, v any) error { return web.Bind(r, v) }

// Validate runs validator on v and returns any [*ValidationError]. Wrapper
// around [web.Validate].
func Validate(v any) error { return web.Validate(v) }

// Render writes a rendered template into the HTTP response, wrapping it in
// the layout for full-page requests. Wrapper around [web.Render].
func Render(w http.ResponseWriter, r *http.Request, statusCode int, name string, data map[string]any) error {
	return web.Render(w, r, statusCode, name, data)
}

// RenderError writes a typed error response (JSON or HTML based on Accept).
// Wrapper around [web.RenderError].
func RenderError(w http.ResponseWriter, r *http.Request, code int, message string) {
	web.RenderError(w, r, code, message)
}

// RenderContent writes pre-rendered template.HTML into the response. Wrapper
// around [web.RenderContent].
func RenderContent(w http.ResponseWriter, r *http.Request, statusCode int, content template.HTML, data map[string]any) error {
	return web.RenderContent(w, r, statusCode, content, data)
}

// RenderFragment executes a named template into template.HTML using the
// context's executor. Wrapper around [web.RenderFragment].
func RenderFragment(ctx context.Context, name string, data map[string]any) (template.HTML, error) {
	return web.RenderFragment(ctx, name, data)
}

// WithLayout stores the layout template name in the context. Wrapper around
// [app.WithLayout].
func WithLayout(ctx context.Context, name string) context.Context {
	return app.WithLayout(ctx, name)
}

// Layout retrieves the layout template name from the context. Wrapper around
// [app.Layout].
func Layout(ctx context.Context) string { return app.Layout(ctx) }

// WithNavItems stores navigation items in the context. Wrapper around
// [app.WithNavItems].
func WithNavItems(ctx context.Context, items []NavItem) context.Context {
	return app.WithNavItems(ctx, items)
}

// NavItems retrieves the navigation items from the context. Wrapper around
// [app.NavItems].
func NavItems(ctx context.Context) []NavItem { return app.NavItems(ctx) }

// WithTemplateExecutor stores the template executor in the context. Wrapper
// around [app.WithTemplateExecutor].
func WithTemplateExecutor(ctx context.Context, exec TemplateExecutor) context.Context {
	return app.WithTemplateExecutor(ctx, exec)
}

// TemplateExec retrieves the template executor from the context. Wrapper
// around [app.TemplateExec].
func TemplateExec(ctx context.Context) TemplateExecutor { return app.TemplateExec(ctx) }

// TemplateExecutorFromContext is a deprecated alias for [TemplateExec].
//
//go:fix inline
func TemplateExecutorFromContext(ctx context.Context) TemplateExecutor {
	return app.TemplateExec(ctx)
}

// WithAuthChecker stores an AuthChecker in the context. Wrapper around
// [app.WithAuthChecker].
func WithAuthChecker(ctx context.Context, checker AuthChecker) context.Context {
	return app.WithAuthChecker(ctx, checker)
}

// WithRequestPath stores the request path in the context. Wrapper around
// [app.WithRequestPath].
func WithRequestPath(ctx context.Context, path string) context.Context {
	return app.WithRequestPath(ctx, path)
}

// RequestPath retrieves the request path from the context. Wrapper around
// [app.RequestPath].
func RequestPath(ctx context.Context) string { return app.RequestPath(ctx) }

// IsAuthenticated returns true if the AuthChecker in context reports
// authentication. Wrapper around [app.IsAuthenticated].
func IsAuthenticated(ctx context.Context) bool { return app.IsAuthenticated(ctx) }

// IsStaff returns true if the AuthChecker in context reports staff status.
// Wrapper around [app.IsStaff].
func IsStaff(ctx context.Context) bool { return app.IsStaff(ctx) }

// IsAdmin returns true if the AuthChecker in context reports admin status.
// Wrapper around [app.IsAdmin].
func IsAdmin(ctx context.Context) bool { return app.IsAdmin(ctx) }

// WithContextValue stores a value in the context under the given key. Wrapper
// around [app.WithContextValue].
func WithContextValue(ctx context.Context, key, val any) context.Context {
	return app.WithContextValue(ctx, key, val)
}

// ContextValue retrieves a typed value from the context. Wrapper around
// [app.ContextValue].
func ContextValue[T any](ctx context.Context, key any) (T, bool) {
	return app.ContextValue[T](ctx, key)
}
