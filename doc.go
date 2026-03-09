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
// [ParsePageRequest] extracts limit, cursor, and page from the query string.
// Use [ApplyCursor] + [TrimCursorResults] + [CursorResult] for cursor-based
// pagination, or [ApplyOffset] + [OffsetResult] for traditional page numbers.
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
//   - htmx — HTMX asset serving and request/response helpers
//   - jobs — SQLite-backed in-process job queue with retry
//   - uploads — pluggable file upload storage
//   - messages — flash messages via session storage
//   - ratelimit — per-client token bucket rate limiting
//   - healthcheck — /healthz endpoint
//   - staticfiles — static file serving with content-hashed URLs
package burrow
