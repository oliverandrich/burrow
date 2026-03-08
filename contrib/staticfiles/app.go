package staticfiles

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
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
func New(fsys fs.FS, opts ...Option) (*App, error) {
	a := &App{
		fsys:   fsys,
		prefix: "/static/",
	}
	for _, opt := range opts {
		opt(a)
	}

	manifest, files, err := buildManifest(fsys)
	if err != nil {
		return nil, fmt.Errorf("build manifest: %w", err)
	}
	a.manifest = manifest
	a.hfs = &hashedFS{fsys: fsys, files: files}

	return a, nil
}

func (a *App) Name() string { return "staticfiles" }

func (a *App) Register(cfg *burrow.AppConfig) error {
	if cfg.Registry == nil {
		return nil
	}
	for _, app := range cfg.Registry.Apps() {
		if provider, ok := app.(burrow.HasStaticFiles); ok {
			prefix, fsys := provider.StaticFS()
			m, f, err := buildManifest(fsys)
			if err != nil {
				return fmt.Errorf("build manifest for %q: %w", prefix, err)
			}
			for orig, hashed := range m {
				a.manifest[prefix+"/"+orig] = prefix + "/" + hashed
			}
			a.hfs.contribs = append(a.hfs.contribs, contribSource{
				prefix: prefix,
				fsys:   fsys,
				files:  f,
			})
		}
	}
	return nil
}

func (a *App) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{a.contextMiddleware, a.cacheHeadersMiddleware}
}

func (a *App) Routes(r chi.Router) {
	r.Handle(a.prefix+"*", http.StripPrefix(a.prefix, http.FileServer(http.FS(a.hfs))))
}

func (a *App) FuncMap() template.FuncMap {
	return template.FuncMap{
		"staticURL": func(name string) string {
			if hashed, exists := a.manifest[name]; exists {
				return a.prefix + hashed
			}
			return a.prefix + name
		},
	}
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
