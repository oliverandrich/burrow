package uploads

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/urfave/cli/v3"
)

const (
	defaultBaseDir   = "data"
	defaultDirName   = "uploads"
	defaultURLPrefix = "/uploads/"
)

// App implements the uploads contrib app for file storage and serving.
type App struct {
	dir          string
	urlPrefix    string
	storage      Store
	allowedTypes []string
}

// Option configures the uploads app.
type Option func(*App)

// WithBaseDir sets the base directory for uploads. The upload directory
// becomes {baseDir}/uploads. Defaults to "data" (i.e. data/uploads).
func WithBaseDir(dir string) Option {
	return func(a *App) {
		a.dir = filepath.Join(dir, defaultDirName)
	}
}

// WithURLPrefix sets the URL prefix for serving uploaded files.
func WithURLPrefix(prefix string) Option {
	return func(a *App) {
		a.urlPrefix = prefix
	}
}

// WithAllowedTypes sets the default allowed MIME types for uploads.
// Per-call StoreOptions.AllowedTypes overrides this default.
func WithAllowedTypes(types ...string) Option {
	return func(a *App) {
		a.allowedTypes = types
	}
}

// New creates an uploads app with sensible defaults:
//   - Upload directory: ./uploads (relative to working directory)
//   - URL prefix: /uploads/
//   - Allowed types: all
//
// Use options to customize, or override at runtime via CLI flags
// (--upload-dir, --upload-url-prefix, --upload-allowed-types).
func New(opts ...Option) *App {
	a := &App{
		dir:       filepath.Join(defaultBaseDir, defaultDirName),
		urlPrefix: defaultURLPrefix,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *App) Name() string { return "uploads" }

func (a *App) Register(_ *burrow.AppConfig) error { return nil }

func (a *App) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "uploads-dir",
			Value:   a.dir,
			Usage:   "Directory for uploaded files",
			Sources: burrow.FlagSources(configSource, "UPLOADS_DIR", "uploads.dir"),
		},
		&cli.StringFlag{
			Name:    "uploads-url-prefix",
			Value:   a.urlPrefix,
			Usage:   "URL prefix for serving uploaded files",
			Sources: burrow.FlagSources(configSource, "UPLOADS_URL_PREFIX", "uploads.url_prefix"),
		},
		&cli.StringFlag{
			Name:    "uploads-allowed-types",
			Usage:   "Comma-separated list of allowed MIME types (empty = all)",
			Sources: burrow.FlagSources(configSource, "UPLOADS_ALLOWED_TYPES", "uploads.allowed_types"),
		},
	}
}

func (a *App) Configure(cmd *cli.Command) error {
	a.dir = cmd.String("uploads-dir")
	a.urlPrefix = cmd.String("uploads-url-prefix")

	if v := cmd.String("uploads-allowed-types"); v != "" {
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

// Store returns the configured Store backend.
func (a *App) Store() Store {
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
