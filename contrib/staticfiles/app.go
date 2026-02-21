package staticfiles

import (
	"context"
	"io/fs"
	"net/http"
	"strings"

	"codeberg.org/oliverandrich/burrow"
	"github.com/go-chi/chi/v5"
)

// App implements the staticfiles contrib app for serving static assets.
type App struct {
	manifest map[string]string // original path → hashed path
	fsys     fs.FS
	hfs      *hashedFS // serves hashed URLs from original files
	prefix   string
}

// Option configures the staticfiles app.
type Option func(*App)

// WithPrefix sets the URL prefix for static files (default: "/static/").
func WithPrefix(prefix string) Option {
	return func(a *App) {
		a.prefix = prefix
	}
}

// New creates a staticfiles app that serves files from the given filesystem.
// It walks the FS at creation time, computing content hashes for all files.
func New(fsys fs.FS, opts ...Option) *App {
	a := &App{
		fsys:   fsys,
		prefix: "/static/",
	}
	for _, opt := range opts {
		opt(a)
	}

	manifest, files := buildManifest(fsys)
	a.manifest = manifest
	a.hfs = &hashedFS{fsys: fsys, files: files}

	return a
}

func (a *App) Name() string                       { return "staticfiles" }
func (a *App) Register(_ *burrow.AppConfig) error { return nil }

func (a *App) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{a.contextMiddleware, a.cacheHeadersMiddleware}
}

func (a *App) Routes(r chi.Router) {
	r.Handle(a.prefix+"*", http.StripPrefix(a.prefix, http.FileServer(http.FS(a.hfs))))
}

func (a *App) contextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), ctxKeyApp{}, a)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) cacheHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, a.prefix) {
			if isHashedAsset(path) {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			}
		}
		next.ServeHTTP(w, r)
	})
}
