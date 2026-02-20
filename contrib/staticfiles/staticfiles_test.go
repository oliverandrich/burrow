package staticfiles

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"codeberg.org/oliverandrich/burrow"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ burrow.App           = (*App)(nil)
	_ burrow.HasMiddleware = (*App)(nil)
	_ burrow.HasRoutes     = (*App)(nil)
)

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

func TestAppName(t *testing.T) {
	app := New(testFS)
	assert.Equal(t, "staticfiles", app.Name())
}

func TestRegisterIsNoop(t *testing.T) {
	app := New(testFS)
	err := app.Register(&burrow.AppConfig{})
	require.NoError(t, err)
}

func TestStaticFileServing(t *testing.T) {
	app := New(testFS)

	e := echo.New()
	app.Routes(e)

	// Compute expected hash for "body{}"
	hash := contentHash([]byte("body{}"))
	hashedPath := "/static/dist/styles." + hash + ".css"

	req := httptest.NewRequest(http.MethodGet, hashedPath, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "body{}", rec.Body.String())
}

func TestStaticFileServingFallback(t *testing.T) {
	// Externally-hashed files (e.g. esbuild font output) should be served directly.
	app := New(testFSWithExternal)

	e := echo.New()
	app.Routes(e)

	req := httptest.NewRequest(http.MethodGet, "/static/dist/bootstrap-icons-CVBWLLHT.woff2", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "font-data", rec.Body.String())
}

func TestCacheHeadersHashedAsset(t *testing.T) {
	app := New(testFS)

	e := echo.New()
	for _, mw := range app.Middleware() {
		e.Use(mw)
	}
	e.GET("/static/*", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	hash := contentHash([]byte("body{}"))
	req := httptest.NewRequest(http.MethodGet, "/static/dist/styles."+hash+".css", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
}

func TestCacheHeadersUnhashedAsset(t *testing.T) {
	app := New(testFS)

	e := echo.New()
	for _, mw := range app.Middleware() {
		e.Use(mw)
	}
	e.GET("/static/*", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/static/dist/styles.css", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "no-cache, no-store, must-revalidate", rec.Header().Get("Cache-Control"))
}

func TestCacheHeadersNonStaticPath(t *testing.T) {
	app := New(testFS)

	e := echo.New()
	for _, mw := range app.Middleware() {
		e.Use(mw)
	}
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Cache-Control"))
}

func TestCustomPrefix(t *testing.T) {
	app := New(testFS, WithPrefix("/assets/"))

	e := echo.New()
	for _, mw := range app.Middleware() {
		e.Use(mw)
	}
	app.Routes(e)

	hash := contentHash([]byte("body{}"))

	// File should be served at the custom prefix.
	req := httptest.NewRequest(http.MethodGet, "/assets/dist/styles."+hash+".css", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "body{}", rec.Body.String())
	assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))

	// Old prefix should not serve files.
	req = httptest.NewRequest(http.MethodGet, "/static/dist/styles."+hash+".css", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestIsHashedAsset(t *testing.T) {
	tests := []struct {
		path   string
		hashed bool
	}{
		{"/static/dist/styles.abcd1234.css", true},
		{"/static/dist/app.EFGH5678.js", true},
		{"/static/dist/icons.A1B2C3D4.woff2", true},
		{"/static/dist/styles.css", false},
		{"/static/dist/app.js", false},
		{"/test", false},
		{"/static/dist/styles.short.css", false},     // hash too short
		{"/static/dist/styles.toolongha.css", false}, // hash too long
		{"/static/dist/styles.abcd-234.css", false},  // non-alphanumeric
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.hashed, isHashedAsset(tt.path))
		})
	}
}

// --- Tests for content hashing helpers ---

func TestContentHash(t *testing.T) {
	hash := contentHash([]byte("body{}"))

	// Must be exactly 8 hex characters.
	assert.Len(t, hash, 8)
	for _, c := range hash {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"expected hex char, got %c", c)
	}

	// Deterministic: same input produces same hash.
	assert.Equal(t, hash, contentHash([]byte("body{}")))

	// Different content produces different hash.
	assert.NotEqual(t, hash, contentHash([]byte("other")))
}

func TestHashedName(t *testing.T) {
	tests := []struct {
		path string
		hash string
		want string
	}{
		{"dist/styles.css", "a1b2c3d4", "dist/styles.a1b2c3d4.css"},
		{"dist/app.js", "deadbeef", "dist/app.deadbeef.js"},
		{"dist/styles.min.css", "abcd1234", "dist/styles.min.abcd1234.css"},
		{"noext", "12345678", "noext.12345678"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, hashedName(tt.path, tt.hash))
		})
	}
}

func TestBuildManifest(t *testing.T) {
	fsys := fstest.MapFS{
		"dist/styles.css": &fstest.MapFile{Data: []byte("body{}")},
		"dist/app.js":     &fstest.MapFile{Data: []byte("alert(1)")},
	}

	manifest, files := buildManifest(fsys)

	// manifest maps original → hashed
	stylesHash := contentHash([]byte("body{}"))
	appHash := contentHash([]byte("alert(1)"))

	assert.Equal(t, "dist/styles."+stylesHash+".css", manifest["dist/styles.css"])
	assert.Equal(t, "dist/app."+appHash+".js", manifest["dist/app.js"])

	// files is the reverse mapping
	assert.Equal(t, "dist/styles.css", files["dist/styles."+stylesHash+".css"])
	assert.Equal(t, "dist/app.js", files["dist/app."+appHash+".js"])
}

func TestBuildManifestDeterministic(t *testing.T) {
	fsys := fstest.MapFS{
		"dist/styles.css": &fstest.MapFile{Data: []byte("body{}")},
	}

	m1, _ := buildManifest(fsys)
	m2, _ := buildManifest(fsys)

	assert.Equal(t, m1, m2)
}

func TestHashedFSOpen(t *testing.T) {
	fsys := fstest.MapFS{
		"dist/styles.css": &fstest.MapFile{Data: []byte("body{}")},
	}

	hash := contentHash([]byte("body{}"))
	files := map[string]string{
		"dist/styles." + hash + ".css": "dist/styles.css",
	}

	hfs := &hashedFS{fsys: fsys, files: files}

	// Opening via hashed name should work.
	f, err := hfs.Open("dist/styles." + hash + ".css")
	require.NoError(t, err)
	f.Close()
}

func TestHashedFSFallback(t *testing.T) {
	fsys := fstest.MapFS{
		"dist/bootstrap-icons-CVBWLLHT.woff2": &fstest.MapFile{Data: []byte("font")},
	}

	hfs := &hashedFS{fsys: fsys, files: map[string]string{}}

	// Not in reverse map — should fall through to underlying FS.
	f, err := hfs.Open("dist/bootstrap-icons-CVBWLLHT.woff2")
	require.NoError(t, err)
	f.Close()
}

func TestHashedFSNotFound(t *testing.T) {
	fsys := fstest.MapFS{}
	hfs := &hashedFS{fsys: fsys, files: map[string]string{}}

	_, err := hfs.Open("dist/nonexistent.css")
	assert.Error(t, err)
}

// --- Tests for URL() template helper ---

func TestURL(t *testing.T) {
	app := New(testFS)
	ctx := context.WithValue(context.Background(), ctxKeyApp{}, app)

	hash := contentHash([]byte("body{}"))
	assert.Equal(t, "/static/dist/styles."+hash+".css", URL(ctx, "dist/styles.css"))
}

func TestURLFallback(t *testing.T) {
	app := New(testFS)
	ctx := context.WithValue(context.Background(), ctxKeyApp{}, app)

	// Unknown file returns prefix + name as-is.
	assert.Equal(t, "/static/dist/unknown.css", URL(ctx, "dist/unknown.css"))
}

func TestURLWithoutContext(t *testing.T) {
	// No app in context — returns name as-is (safe fallback, no prefix).
	ctx := context.Background()
	assert.Equal(t, "dist/styles.css", URL(ctx, "dist/styles.css"))
}

func TestURLCustomPrefix(t *testing.T) {
	app := New(testFS, WithPrefix("/assets/"))
	ctx := context.WithValue(context.Background(), ctxKeyApp{}, app)

	hash := contentHash([]byte("body{}"))
	assert.Equal(t, "/assets/dist/styles."+hash+".css", URL(ctx, "dist/styles.css"))
}

func TestMiddlewareInjectsContext(t *testing.T) {
	app := New(testFS)

	e := echo.New()
	for _, mw := range app.Middleware() {
		e.Use(mw)
	}

	var gotURL string
	e.GET("/test", func(c *echo.Context) error {
		gotURL = URL(c.Request().Context(), "dist/styles.css")
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	hash := contentHash([]byte("body{}"))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/static/dist/styles."+hash+".css", gotURL)
}
