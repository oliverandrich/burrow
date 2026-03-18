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
	assert.Equal(t, "bootstrap", New().Name())
}

func TestAppRegister(t *testing.T) {
	require.NoError(t, New().Register(&burrow.AppConfig{}))
}

func TestDefaultColor(t *testing.T) {
	app := New()
	assert.Equal(t, Purple, app.color)
}

func TestWithColor(t *testing.T) {
	app := New(WithColor(Blue))
	assert.Equal(t, Blue, app.color)
}

func TestDependencies(t *testing.T) {
	app := New()
	assert.Equal(t, []string{"staticfiles", "htmx"}, app.Dependencies())
}

func TestStaticFS(t *testing.T) {
	prefix, fsys := New().StaticFS()
	assert.Equal(t, "bootstrap", prefix)
	require.NotNil(t, fsys)

	for _, name := range []string{
		"bootstrap.min.css",
		"bootstrap.bundle.min.js",
		"theme-blue.min.css",
		"theme-purple.min.css",
		"theme-gray.min.css",
	} {
		f, err := fsys.Open(name)
		require.NoError(t, err, "expected %s to exist in static FS", name)
		_ = f.Close()
	}
}

func TestTemplateFS(t *testing.T) {
	fsys := New().TemplateFS()
	require.NotNil(t, fsys)

	for _, name := range []string{
		"layout.html",
		"nav_layout.html",
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
	fm := New().FuncMap()

	for _, key := range []string{
		"iconSunFill", "iconMoonStarsFill", "iconCircleHalf",
	} {
		assert.Contains(t, fm, key)
	}
}

func TestCSSTemplateReturnsCorrectPath(t *testing.T) {
	tests := []struct {
		color    Color
		expected string
	}{
		{Default, "bootstrap/bootstrap.min.css"},
		{Blue, "bootstrap/theme-blue.min.css"},
		{Purple, "bootstrap/theme-purple.min.css"},
		{Gray, "bootstrap/theme-gray.min.css"},
	}
	for _, tt := range tests {
		app := New(WithColor(tt.color))
		assert.Contains(t, app.cssTemplate(), tt.expected)
	}
}

func TestCSSTemplateCustom(t *testing.T) {
	app := New(WithCustomCSS("myapp/mytheme.min.css"))
	assert.Contains(t, app.cssTemplate(), "myapp/mytheme.min.css")
}

func TestWithCustomCSSOverridesColor(t *testing.T) {
	app := New(WithColor(Blue), WithCustomCSS("myapp/custom.css"))
	assert.Contains(t, app.cssTemplate(), "myapp/custom.css")
}

func TestWithColorClearsCustomCSS(t *testing.T) {
	app := New(WithCustomCSS("myapp/custom.css"), WithColor(Gray))
	assert.Contains(t, app.cssTemplate(), "bootstrap/theme-gray.min.css")
}

func TestFuncMapIconsReturnSVG(t *testing.T) {
	fm := New().FuncMap()

	for _, key := range []string{"iconSunFill", "iconMoonStarsFill", "iconCircleHalf"} {
		fn := fm[key].(func(...string) template.HTML)
		svg := fn()
		assert.Contains(t, string(svg), "<svg", "expected %s to return SVG", key)
	}
}

func TestFuncMapIconsAcceptClasses(t *testing.T) {
	fn := New().FuncMap()["iconSunFill"].(func(...string) template.HTML)
	svg := fn("fs-1", "d-block")
	assert.Contains(t, string(svg), `class="fs-1 d-block"`)
}

func TestMiddlewareInjectsLayout(t *testing.T) {
	mws := New().Middleware()
	require.Len(t, mws, 1)

	var got string
	handler := mws[0](http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = burrow.Layout(r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), newGetRequest())
	assert.Equal(t, "bootstrap/layout", got)
}

func TestMiddlewareDoesNotOverride(t *testing.T) {
	mws := New().Middleware()

	var got string
	handler := mws[0](http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = burrow.Layout(r.Context())
	}))
	req := newGetRequest()
	req = req.WithContext(burrow.WithLayout(req.Context(), "custom/layout"))
	handler.ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "custom/layout", got)
}

func TestLayoutReturnsTemplateName(t *testing.T) {
	assert.Equal(t, "bootstrap/layout", Layout())
}

func TestNavLayoutReturnsTemplateName(t *testing.T) {
	assert.Equal(t, "bootstrap/nav_layout", NavLayout())
}
