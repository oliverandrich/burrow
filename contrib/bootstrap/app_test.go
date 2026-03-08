package bootstrap

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGetRequest() *http.Request {
	return httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
}

// Compile-time interface assertions.
var (
	_ burrow.App             = (*App)(nil)
	_ burrow.HasStaticFiles  = (*App)(nil)
	_ burrow.HasMiddleware   = (*App)(nil)
	_ burrow.HasTemplates    = (*App)(nil)
	_ burrow.HasFuncMap      = (*App)(nil)
	_ burrow.HasDependencies = (*App)(nil)
)

func TestAppName(t *testing.T) {
	app := New()
	assert.Equal(t, "bootstrap", app.Name())
}

func TestAppRegister(t *testing.T) {
	app := New()
	err := app.Register(&burrow.AppConfig{})
	require.NoError(t, err)
}

func TestDependencies(t *testing.T) {
	app := New()
	assert.Equal(t, []string{"staticfiles", "htmx"}, app.Dependencies())
}

func TestStaticFS(t *testing.T) {
	app := New()
	prefix, fsys := app.StaticFS()

	assert.Equal(t, "bootstrap", prefix)
	require.NotNil(t, fsys)

	for _, name := range []string{
		"bootstrap.min.css",
		"bootstrap.bundle.min.js",
	} {
		f, err := fsys.Open(name)
		require.NoError(t, err, "expected %s to exist in static FS", name)
		_ = f.Close()
	}
}

func TestTemplateFS(t *testing.T) {
	app := New()
	fsys := app.TemplateFS()
	require.NotNil(t, fsys)

	for _, name := range []string{
		"layout.html",
		"theme_script.html",
		"theme_switcher.html",
		"pagination.html",
	} {
		f, err := fsys.Open(name)
		require.NoError(t, err, "expected %s to exist in template FS", name)
		_ = f.Close()
	}
}

func TestFuncMap(t *testing.T) {
	app := New()
	fm := app.FuncMap()

	expectedKeys := []string{
		"iconSunFill", "iconMoonStarsFill", "iconCircleHalf",
		"pageURL", "pageLimit", "pageNumbers",
		"add", "sub",
	}
	for _, key := range expectedKeys {
		assert.Contains(t, fm, key)
	}
}

func TestFuncMapIconsReturnSVG(t *testing.T) {
	app := New()
	fm := app.FuncMap()

	for _, key := range []string{"iconSunFill", "iconMoonStarsFill", "iconCircleHalf"} {
		fn := fm[key].(func(...string) template.HTML)
		svg := fn()
		assert.Contains(t, string(svg), "<svg", "expected %s to return SVG", key)
	}
}

func TestFuncMapIconsAcceptClasses(t *testing.T) {
	app := New()
	fm := app.FuncMap()

	fn := fm["iconSunFill"].(func(...string) template.HTML)
	svg := fn("fs-1", "d-block")
	assert.Contains(t, string(svg), `class="fs-1 d-block"`)
}

func TestFuncMapArithmetic(t *testing.T) {
	app := New()
	fm := app.FuncMap()

	add := fm["add"].(func(int, int) int)
	sub := fm["sub"].(func(int, int) int)

	assert.Equal(t, 5, add(2, 3))
	assert.Equal(t, 1, sub(3, 2))
}

func TestMiddlewareInjectsLayout(t *testing.T) {
	app := New()
	mws := app.Middleware()
	require.Len(t, mws, 1)

	var got burrow.LayoutFunc
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = burrow.Layout(r.Context())
	})

	handler := mws[0](inner)

	req := newGetRequest()
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.NotNil(t, got, "middleware should inject layout when none is set")
}

func TestMiddlewareDoesNotOverride(t *testing.T) {
	app := New()
	mws := app.Middleware()
	require.Len(t, mws, 1)

	sentinel := burrow.LayoutFunc(func(w http.ResponseWriter, _ *http.Request, code int, content template.HTML, _ map[string]any) error {
		return burrow.HTML(w, code, "sentinel:"+string(content))
	})

	var got burrow.LayoutFunc
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = burrow.Layout(r.Context())
	})

	handler := mws[0](inner)

	req := newGetRequest()
	req = req.WithContext(burrow.WithLayout(req.Context(), sentinel))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.NotNil(t, got, "layout should still be set")

	// Verify it's the original sentinel, not bootstrap's layout.
	rec := httptest.NewRecorder()
	err := got(rec, newGetRequest(), http.StatusOK, "test", nil)
	require.NoError(t, err)
	assert.Equal(t, "sentinel:test", rec.Body.String())
}

func TestLayoutWithoutExecutorFallsBack(t *testing.T) {
	layout := Layout()

	rec := httptest.NewRecorder()
	req := newGetRequest()
	// No template executor in context — should fall back to raw content.
	err := layout(rec, req, http.StatusOK, "<p>hello</p>", nil)
	require.NoError(t, err)
	assert.Equal(t, "<p>hello</p>", rec.Body.String())
}
