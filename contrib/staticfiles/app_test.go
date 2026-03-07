package staticfiles

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"codeberg.org/oliverandrich/burrow"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ burrow.App           = (*App)(nil)
	_ burrow.HasMiddleware = (*App)(nil)
	_ burrow.HasRoutes     = (*App)(nil)
	_ burrow.HasFuncMap    = (*App)(nil)
)

// mustNew is a test helper that creates a staticfiles App and fails the test on error.
func mustNew(t *testing.T, fsys fs.FS, opts ...Option) *App {
	t.Helper()
	app, err := New(fsys, opts...)
	require.NoError(t, err)
	return app
}

// testFS uses unhashed filenames — the App computes content hashes itself.
var testFS = fstest.MapFS{
	"dist/styles.css": &fstest.MapFile{Data: []byte("body{}")},
	"dist/app.js":     &fstest.MapFile{Data: []byte("console.log('test')")},
}

// testFSWithExternal has both app-owned files and externally-hashed files (e.g. esbuild font output).
var testFSWithExternal = fstest.MapFS{
	"dist/styles.css":                     &fstest.MapFile{Data: []byte("body{}")},
	"dist/bootstrap-icons-CVBWLLHT.woff2": &fstest.MapFile{Data: []byte("font-data")},
}

// contribApp is a test app implementing HasStaticFiles.
type contribApp struct {
	fsys   fs.FS
	name   string
	prefix string
}

func (a *contribApp) Name() string                       { return a.name }
func (a *contribApp) Register(_ *burrow.AppConfig) error { return nil }
func (a *contribApp) StaticFS() (string, fs.FS)          { return a.prefix, a.fsys }

func TestNewError(t *testing.T) {
	_, err := New(&brokenFS{err: errors.New("disk on fire")})
	require.Error(t, err)
	assert.ErrorContains(t, err, "disk on fire")
}

func TestRegisterContribFSError(t *testing.T) {
	registry := burrow.NewRegistry()
	registry.Add(&contribApp{
		name:   "broken",
		prefix: "broken",
		fsys:   &brokenFS{err: errors.New("contrib broken")},
	})

	app := mustNew(t, testFS)
	registry.Add(app)
	err := app.Register(&burrow.AppConfig{Registry: registry})
	require.Error(t, err)
	assert.ErrorContains(t, err, "contrib broken")
}

func TestAppName(t *testing.T) {
	app := mustNew(t, testFS)
	assert.Equal(t, "staticfiles", app.Name())
}

func TestRegisterIsNoop(t *testing.T) {
	app := mustNew(t, testFS)
	err := app.Register(&burrow.AppConfig{})
	require.NoError(t, err)
}

func TestStaticFileServing(t *testing.T) {
	app := mustNew(t, testFS)

	r := chi.NewRouter()
	app.Routes(r)

	// Compute expected hash for "body{}"
	hash := contentHash([]byte("body{}"))
	hashedPath := "/static/dist/styles." + hash + ".css"

	req := httptest.NewRequest(http.MethodGet, hashedPath, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "body{}", rec.Body.String())
}

func TestStaticFileServingFallback(t *testing.T) {
	// Externally-hashed files (e.g. esbuild font output) should be served directly.
	app := mustNew(t, testFSWithExternal)

	r := chi.NewRouter()
	app.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/static/dist/bootstrap-icons-CVBWLLHT.woff2", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "font-data", rec.Body.String())
}

func TestCacheHeadersHashedAsset(t *testing.T) {
	app := mustNew(t, testFS)

	r := chi.NewRouter()
	for _, mw := range app.Middleware() {
		r.Use(mw)
	}
	r.Get("/static/*", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	hash := contentHash([]byte("body{}"))
	req := httptest.NewRequest(http.MethodGet, "/static/dist/styles."+hash+".css", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
}

func TestCacheHeadersUnhashedAsset(t *testing.T) {
	app := mustNew(t, testFS)

	r := chi.NewRouter()
	for _, mw := range app.Middleware() {
		r.Use(mw)
	}
	r.Get("/static/*", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/static/dist/styles.css", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "no-cache, no-store, must-revalidate", rec.Header().Get("Cache-Control"))
}

func TestCacheHeadersNonStaticPath(t *testing.T) {
	app := mustNew(t, testFS)

	r := chi.NewRouter()
	for _, mw := range app.Middleware() {
		r.Use(mw)
	}
	r.Get("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Cache-Control"))
}

func TestCustomPrefix(t *testing.T) {
	app := mustNew(t, testFS, WithPrefix("/assets/"))

	r := chi.NewRouter()
	for _, mw := range app.Middleware() {
		r.Use(mw)
	}
	app.Routes(r)

	hash := contentHash([]byte("body{}"))

	// File should be served at the custom prefix.
	req := httptest.NewRequest(http.MethodGet, "/assets/dist/styles."+hash+".css", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "body{}", rec.Body.String())
	assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))

	// Old prefix should not serve files.
	req = httptest.NewRequest(http.MethodGet, "/static/dist/styles."+hash+".css", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestRegisterCollectsContribFS(t *testing.T) {
	adminFS := fstest.MapFS{
		"admin.css": &fstest.MapFile{Data: []byte(".admin{}")},
	}

	registry := burrow.NewRegistry()
	registry.Add(&contribApp{name: "test-admin", prefix: "admin", fsys: adminFS})

	app := mustNew(t, testFS)
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	// The manifest should include both user and contrib files.
	hash := contentHash([]byte(".admin{}"))
	assert.Equal(t, "admin/admin."+hash+".css", app.manifest["admin/admin.css"])
}

func TestContribFileServing(t *testing.T) {
	adminFS := fstest.MapFS{
		"admin.css": &fstest.MapFile{Data: []byte(".admin{}")},
	}

	registry := burrow.NewRegistry()
	registry.Add(&contribApp{name: "test-admin", prefix: "admin", fsys: adminFS})

	app := mustNew(t, testFS)
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	r := chi.NewRouter()
	app.Routes(r)

	hash := contentHash([]byte(".admin{}"))
	req := httptest.NewRequest(http.MethodGet, "/static/admin/admin."+hash+".css", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, ".admin{}", rec.Body.String())
}

func TestContribFileURL(t *testing.T) {
	adminFS := fstest.MapFS{
		"admin.css": &fstest.MapFile{Data: []byte(".admin{}")},
	}

	registry := burrow.NewRegistry()
	registry.Add(&contribApp{name: "test-admin", prefix: "admin", fsys: adminFS})

	app := mustNew(t, testFS)
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	ctx := context.WithValue(context.Background(), ctxKeyApp{}, app)

	hash := contentHash([]byte(".admin{}"))
	assert.Equal(t, "/static/admin/admin."+hash+".css", URL(ctx, "admin/admin.css"))
}

func TestContribAndUserFilesCoexist(t *testing.T) {
	adminFS := fstest.MapFS{
		"admin.css": &fstest.MapFile{Data: []byte(".admin{}")},
	}

	registry := burrow.NewRegistry()
	registry.Add(&contribApp{name: "test-admin", prefix: "admin", fsys: adminFS})

	app := mustNew(t, testFS)
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	r := chi.NewRouter()
	app.Routes(r)

	// User file still works.
	userHash := contentHash([]byte("body{}"))
	req := httptest.NewRequest(http.MethodGet, "/static/dist/styles."+userHash+".css", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "body{}", rec.Body.String())

	// Contrib file works.
	adminHash := contentHash([]byte(".admin{}"))
	req = httptest.NewRequest(http.MethodGet, "/static/admin/admin."+adminHash+".css", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, ".admin{}", rec.Body.String())
}

func TestContribFileCacheHeaders(t *testing.T) {
	adminFS := fstest.MapFS{
		"admin.css": &fstest.MapFile{Data: []byte(".admin{}")},
	}

	registry := burrow.NewRegistry()
	registry.Add(&contribApp{name: "test-admin", prefix: "admin", fsys: adminFS})

	app := mustNew(t, testFS)
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	r := chi.NewRouter()
	for _, mw := range app.Middleware() {
		r.Use(mw)
	}
	app.Routes(r)

	hash := contentHash([]byte(".admin{}"))
	req := httptest.NewRequest(http.MethodGet, "/static/admin/admin."+hash+".css", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
}

func TestNoContribsStillWorks(t *testing.T) {
	registry := burrow.NewRegistry()

	app := mustNew(t, testFS)
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	// User files should still work exactly as before.
	r := chi.NewRouter()
	app.Routes(r)

	hash := contentHash([]byte("body{}"))
	req := httptest.NewRequest(http.MethodGet, "/static/dist/styles."+hash+".css", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "body{}", rec.Body.String())
}

func TestFuncMapStaticURL(t *testing.T) {
	app := mustNew(t, testFS)

	fm := app.FuncMap()
	require.Contains(t, fm, "staticURL")

	staticURL := fm["staticURL"].(func(string) string)

	hash := contentHash([]byte("body{}"))
	assert.Equal(t, "/static/dist/styles."+hash+".css", staticURL("dist/styles.css"))
}

func TestFuncMapStaticURLFallback(t *testing.T) {
	app := mustNew(t, testFS)

	fm := app.FuncMap()
	staticURL := fm["staticURL"].(func(string) string)

	// Unknown file falls back to prefix + name.
	assert.Equal(t, "/static/unknown.css", staticURL("unknown.css"))
}

func TestMultipleContribs(t *testing.T) {
	adminFS := fstest.MapFS{
		"admin.css": &fstest.MapFile{Data: []byte(".admin{}")},
	}
	themeFS := fstest.MapFS{
		"theme.css": &fstest.MapFile{Data: []byte(".theme{}")},
	}

	registry := burrow.NewRegistry()
	registry.Add(&contribApp{name: "test-admin", prefix: "admin", fsys: adminFS})
	registry.Add(&contribApp{name: "test-theme", prefix: "theme", fsys: themeFS})

	app := mustNew(t, testFS)
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	ctx := context.WithValue(context.Background(), ctxKeyApp{}, app)

	adminHash := contentHash([]byte(".admin{}"))
	themeHash := contentHash([]byte(".theme{}"))

	assert.Equal(t, "/static/admin/admin."+adminHash+".css", URL(ctx, "admin/admin.css"))
	assert.Equal(t, "/static/theme/theme."+themeHash+".css", URL(ctx, "theme/theme.css"))
}
