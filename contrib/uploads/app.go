package uploads

import (
	"net/http"
	"strings"

	"codeberg.org/oliverandrich/burrow"
	"github.com/go-chi/chi/v5"
	"github.com/urfave/cli/v3"
)

// App implements the uploads contrib app for file storage and serving.
type App struct {
	dir          string
	urlPrefix    string
	storage      Storage
	allowedTypes []string
}

// Option configures the uploads app.
type Option func(*App)

// WithAllowedTypes sets the default allowed MIME types for uploads.
// Per-call StoreOptions.AllowedTypes overrides this default.
func WithAllowedTypes(types ...string) Option {
	return func(a *App) {
		a.allowedTypes = types
	}
}

// New creates an uploads app with the given default directory and URL prefix.
// These defaults can be overridden by CLI flags (--upload-dir, --upload-url-prefix).
func New(dir, urlPrefix string, opts ...Option) *App {
	a := &App{
		dir:       dir,
		urlPrefix: urlPrefix,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *App) Name() string { return "uploads" }

func (a *App) Register(_ *burrow.AppConfig) error { return nil }

func (a *App) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "upload-dir",
			Usage:   "Directory for uploaded files",
			Sources: cli.EnvVars("UPLOAD_DIR"),
		},
		&cli.StringFlag{
			Name:    "upload-url-prefix",
			Usage:   "URL prefix for serving uploaded files",
			Sources: cli.EnvVars("UPLOAD_URL_PREFIX"),
		},
		&cli.StringFlag{
			Name:    "upload-allowed-types",
			Usage:   "Comma-separated list of allowed MIME types (empty = all)",
			Sources: cli.EnvVars("UPLOAD_ALLOWED_TYPES"),
		},
	}
}

func (a *App) Configure(cmd *cli.Command) error {
	if v := cmd.String("upload-dir"); v != "" {
		a.dir = v
	}
	if v := cmd.String("upload-url-prefix"); v != "" {
		a.urlPrefix = v
	}
	if v := cmd.String("upload-allowed-types"); v != "" {
		a.allowedTypes = strings.Split(v, ",")
		for i := range a.allowedTypes {
			a.allowedTypes[i] = strings.TrimSpace(a.allowedTypes[i])
		}
	}

	s, err := NewLocalStorage(a.dir, a.urlPrefix)
	if err != nil {
		return err
	}
	a.storage = s
	return nil
}

// Storage returns the configured Storage backend.
func (a *App) Storage() Storage {
	return a.storage
}

// AllowedTypes returns the globally configured allowed MIME types.
func (a *App) AllowedTypes() []string {
	return a.allowedTypes
}

func (a *App) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{a.contextMiddleware}
}

func (a *App) Routes(r chi.Router) {
	r.Handle(a.urlPrefix+"*", a.servingHandler())
}

func (a *App) contextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithStorage(r.Context(), a.storage)
		if len(a.allowedTypes) > 0 {
			ctx = withAllowedTypes(ctx, a.allowedTypes)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) servingHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.StripPrefix(a.urlPrefix, http.FileServer(http.Dir(a.dir))).ServeHTTP(w, r)
	})
}

// Compile-time interface assertions.
var (
	_ burrow.App           = (*App)(nil)
	_ burrow.Configurable  = (*App)(nil)
	_ burrow.HasMiddleware = (*App)(nil)
	_ burrow.HasRoutes     = (*App)(nil)
)
