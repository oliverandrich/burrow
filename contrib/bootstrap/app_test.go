package bootstrap

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oliverandrich/burrow"
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
		"css.html",
		"js.html",
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

	var got string
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = burrow.Layout(r.Context())
	})

	handler := mws[0](inner)

	req := newGetRequest()
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "bootstrap/layout", got, "middleware should inject layout when none is set")
}

func TestMiddlewareDoesNotOverride(t *testing.T) {
	app := New()
	mws := app.Middleware()
	require.Len(t, mws, 1)

	var got string
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = burrow.Layout(r.Context())
	})

	handler := mws[0](inner)

	req := newGetRequest()
	req = req.WithContext(burrow.WithLayout(req.Context(), "custom/layout"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "custom/layout", got, "middleware should not override existing layout")
}

func TestLayoutReturnsTemplateName(t *testing.T) {
	assert.Equal(t, "bootstrap/layout", Layout())
}
