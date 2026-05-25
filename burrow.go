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
//   - [github.com/oliverandrich/burrow/server] — Server, OpenDB, boot sequence
//   - [github.com/oliverandrich/burrow/web] — HandlerFunc, HTTPError, Handle,
//     JSON, Render, Bind, Validate
//   - [github.com/oliverandrich/burrow/tasks] — Queue, DefineTask,
//     DefineResultTask, JobOption
//   - [github.com/oliverandrich/burrow/pagination] — PageRequest,
//     ParsePageRequest, OffsetResult, PageURL
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
//   - admin — admin panel coordinator with three-tier role gating
//   - auth — WebAuthn passkey authentication with recovery codes
//   - authmail — pluggable email rendering with SMTP backend
//   - csrf — CSRF protection (gorilla/csrf)
//   - healthcheck — liveness and readiness probes
//   - htmx — HTMX asset serving and request/response helpers
//   - humanize — i18n-aware template helpers for times, numbers, file sizes
//   - jobs — Den-backed in-process job queue with retry (SQLite + Postgres)
//   - messages — flash messages via session storage
//   - ratelimit — per-client token bucket rate limiting
//   - secure — security response headers (X-Frame-Options, HSTS, CSP, …)
//   - selfupdate — in-app binary self-update from GitHub releases
//   - session — cookie-based sessions (gorilla/sessions)
//   - sse — Server-Sent Events with in-memory pub/sub broker
//   - staticfiles — static file serving with content-hashed URLs
//
// Locale detection and translations are provided by the root
// [github.com/oliverandrich/burrow/i18n] package, not a contrib.
package burrow
