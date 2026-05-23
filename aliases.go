package burrow

import (
	"github.com/oliverandrich/burrow/app"
	"github.com/oliverandrich/burrow/pagination"
	"github.com/oliverandrich/burrow/registry"
	"github.com/oliverandrich/burrow/server"
	"github.com/oliverandrich/burrow/tasks"
	"github.com/oliverandrich/burrow/web"
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
