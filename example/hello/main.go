// Command hello is a minimal burrow application that serves a single
// "Hello, World!" page with Bootstrap styling and i18n support.
//
// This example demonstrates the core concepts of a burrow app:
//
//   - Creating a server with contrib apps (i18n, staticfiles, bootstrap)
//   - Defining a custom app that provides routes, templates, and translations
//   - Using the Bootstrap layout for page rendering
//   - Configuring the CLI with urfave/cli for flags like --host, --port, etc.
//
// Run it with:
//
//	go run ./example/hello
//
// Then open http://localhost:8080 in your browser.
package main

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/bootstrap"
	"codeberg.org/oliverandrich/burrow/contrib/i18n"
	"codeberg.org/oliverandrich/burrow/contrib/staticfiles"
	"github.com/go-chi/chi/v5"
	"github.com/urfave/cli/v3"
)

// Embed the templates/ and translations/ directories at compile time.
// Go's //go:embed directive makes these files available as an in-memory
// filesystem, so the binary is fully self-contained — no external files needed.

//go:embed templates
var templateFS embed.FS

//go:embed translations
var translationFS embed.FS

// emptyFS is an empty embedded filesystem. We pass it to staticfiles because
// this example has no custom static assets (CSS, JS, images). Contrib apps
// like bootstrap contribute their own static files automatically.
var emptyFS embed.FS

func main() {
	// staticfiles serves static assets with content-hashed URLs for cache busting.
	// Even though we have no custom assets, we need it because bootstrap depends on it.
	staticApp, err := staticfiles.New(emptyFS)
	if err != nil {
		log.Fatal(err)
	}

	// Create our custom app. The helloApp struct is defined below and
	// provides a single route, HTML templates, and translation files.
	hello := &helloApp{}

	// Create the server and register apps. Order matters: apps are initialized
	// in the order they are passed, and some apps depend on others being
	// registered first (e.g. bootstrap depends on staticfiles).
	srv := burrow.NewServer(
		i18n.New(),      // Locale detection middleware, loads translation files
		staticApp,       // Static file serving with content-hashed URLs
		bootstrap.New(), // Bootstrap 5 CSS/JS, htmx, and dark mode theme switcher
		hello,           // Our custom app (defined below)
	)

	// SetLayout wraps every page in the given layout template. The bootstrap
	// contrib provides a ready-made layout with <head>, Bootstrap CSS/JS, and
	// a theme switcher. For a custom layout with navbar, see the notes example.
	srv.SetLayout(bootstrap.Layout())

	// Wire up the CLI. The server provides built-in flags (--host, --port,
	// --database-dsn, --log-level, etc.) and the Action runs the HTTP server.
	cmd := &cli.Command{
		Name:   "hello",
		Usage:  "Minimal burrow hello world application",
		Flags:  srv.Flags(nil),
		Action: srv.Run,
	}

	// Parse CLI flags (os.Args) and start the HTTP server. This blocks
	// until the server shuts down (e.g. via Ctrl+C / SIGTERM).
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// helloApp — a minimal custom app
// ---------------------------------------------------------------------------
//
// Every burrow app must implement the burrow.App interface:
//
//	Name() string                   — unique identifier for the app
//	Register(cfg *AppConfig) error  — called once during server boot
//
// Beyond that, apps opt into additional features by implementing optional
// interfaces. This app implements:
//
//	HasRoutes       — registers HTTP routes
//	HasTemplates    — contributes HTML templates to the global template set
//	HasFuncMap      — contributes template functions (none here, but required)
//	HasTranslations — contributes i18n translation files

type helloApp struct{}

func (a *helloApp) Name() string { return "hello" }

// Register is called during server boot with the app's configuration
// (database, logger, etc.). This simple app doesn't need any setup.
func (a *helloApp) Register(_ *burrow.AppConfig) error { return nil }

// TranslationFS returns the embedded translations/ directory. The i18n app
// automatically discovers and loads all .toml files from this filesystem.
// Files are named active.<lang>.toml (e.g. active.en.toml, active.de.toml).
func (a *helloApp) TranslationFS() fs.FS { return translationFS }

// TemplateFS returns the embedded HTML templates. Templates are organized
// in subdirectories named after the app (templates/hello/*.html) and use
// Go's html/template syntax. The {{ define "hello/home" }} directive sets
// the template name used when rendering.
func (a *helloApp) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return sub
}

// FuncMap returns custom template functions. This app doesn't need any,
// but the interface must be satisfied. Other apps use this to provide
// icon helpers, formatting functions, etc.
func (a *helloApp) FuncMap() template.FuncMap { return nil }

// Routes registers HTTP routes on the given chi router. burrow.Handle()
// wraps a handler that returns an error into a standard http.HandlerFunc,
// providing centralized error handling via HTTPError.
//
// RenderTemplate looks up the named template ("hello/home"), executes it
// with the given data, and wraps it in the server's layout. For HTMX
// requests, it automatically returns only the fragment (no layout).
func (a *helloApp) Routes(r chi.Router) {
	r.Get("/", burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return burrow.RenderTemplate(w, r, http.StatusOK, "hello/home", map[string]any{
			"Title": "Hello",
		})
	}))
}
