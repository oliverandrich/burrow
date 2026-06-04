// Package apidocs serves a self-hosted, interactive OpenAPI documentation UI.
//
// It vendors the Scalar API reference as a single standalone bundle and renders
// a page that points the viewer at an OpenAPI spec URL — by default the spec
// produced by the crud package's API.SpecHandler. The bundle is served through
// the staticfiles contrib (content-hashed), and web fonts are disabled, so the
// UI works fully offline with no CDN dependency.
//
// Mount it by registering the app and (optionally) overriding the spec URL,
// route, or page title:
//
//	srv := burrow.NewServer(
//		staticfiles.New(...),
//		apidocs.New(apidocs.WithSpecURL("/api/openapi.json"), apidocs.WithTitle("My API")),
//	)
//
// The app serves the documentation page at its route (default /api/docs).
package apidocs

import (
	"embed"
	"io/fs"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
)

//go:embed static
var staticFS embed.FS

//go:embed templates
var templateFS embed.FS

const (
	defaultSpecURL = "/api/openapi.json"
	defaultRoute   = "/api/docs"
	defaultTitle   = "API Documentation"
)

// App is the burrow contrib app that serves the Scalar OpenAPI documentation
// UI. Build it with [New].
type App struct {
	specURL string
	route   string
	title   string
}

// Option configures the [App].
type Option func(*App)

// WithSpecURL sets the URL the viewer loads the OpenAPI spec from.
// Defaults to /api/openapi.json (the conventional crud SpecHandler route).
func WithSpecURL(url string) Option { return func(a *App) { a.specURL = url } }

// WithRoute sets the route the documentation page is served at.
// Defaults to /api/docs.
func WithRoute(route string) Option { return func(a *App) { a.route = route } }

// WithTitle sets the HTML page title. Defaults to "API Documentation".
func WithTitle(title string) Option { return func(a *App) { a.title = title } }

// New creates an apidocs app with the given options.
func New(opts ...Option) *App {
	a := &App{specURL: defaultSpecURL, route: defaultRoute, title: defaultTitle}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *App) Name() string { return "apidocs" }

// Dependencies ensures the staticfiles app is present — the vendored Scalar
// bundle is served content-hashed through it.
func (a *App) Dependencies() []string { return []string{"staticfiles"} }

// TemplateFS returns the embedded documentation page template.
func (a *App) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return sub
}

// StaticFS returns the embedded Scalar bundle under the "apidocs" prefix.
func (a *App) StaticFS() (string, fs.FS) {
	sub, _ := fs.Sub(staticFS, "static")
	return "apidocs", sub
}

// Routes registers the documentation page at the configured route.
func (a *App) Routes(r chi.Router) {
	r.Get(a.route, burrow.Handle(a.handlePage))
}
