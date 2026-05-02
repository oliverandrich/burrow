package pico

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
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
	_ burrow.HasDependencies = (*App)(nil)
)

func TestAppName(t *testing.T) {
	assert.Equal(t, "pico", New().Name())
}

func TestDefaultColor(t *testing.T) {
	app := New()
	assert.Equal(t, Default, app.color)
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
	assert.Equal(t, "pico", prefix)
	require.NotNil(t, fsys)

	for _, c := range AllColors() {
		name := "pico.min.css"
		if c != Default {
			name = "pico." + string(c) + ".min.css"
		}
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
	} {
		f, err := fsys.Open(name)
		require.NoError(t, err, "expected %s to exist in template FS", name)
		_ = f.Close()
	}
}

func TestCSSTemplateReturnsCorrectPath(t *testing.T) {
	tests := []struct {
		color    Color
		expected string
	}{
		{Default, "pico/pico.min.css"},
		{Blue, "pico/pico.blue.min.css"},
		{Purple, "pico/pico.purple.min.css"},
		{Zinc, "pico/pico.zinc.min.css"},
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
	app := New(WithCustomCSS("myapp/custom.css"), WithColor(Zinc))
	assert.Contains(t, app.cssTemplate(), "pico/pico.zinc.min.css")
}

func TestNoCompactByDefault(t *testing.T) {
	app := New()
	assert.NotContains(t, app.cssTemplate(), "pico-compact.min.css")
}

func TestWithCompactType(t *testing.T) {
	app := New(WithCompactType())
	tpl := app.cssTemplate()
	assert.Contains(t, tpl, "pico/pico.min.css")
	assert.Contains(t, tpl, "pico/pico-compact.min.css")
	assert.Greater(t, strings.Index(tpl, "pico/pico-compact.min.css"),
		strings.Index(tpl, "pico/pico.min.css"),
		"compact override must follow the main stylesheet so source-order cascade applies")
}

func TestCompactCombinesWithColor(t *testing.T) {
	app := New(WithColor(Blue), WithCompactType())
	tpl := app.cssTemplate()
	assert.Contains(t, tpl, "pico/pico.blue.min.css")
	assert.Contains(t, tpl, "pico/pico-compact.min.css")
	assert.Greater(t, strings.Index(tpl, "pico/pico-compact.min.css"),
		strings.Index(tpl, "pico/pico.blue.min.css"),
		"compact override must follow the color stylesheet so source-order cascade applies")
}

func TestCompactIgnoredWithCustomCSS(t *testing.T) {
	app := New(WithCustomCSS("myapp/custom.css"), WithCompactType())
	tpl := app.cssTemplate()
	assert.Contains(t, tpl, "myapp/custom.css")
	assert.NotContains(t, tpl, "pico-compact.min.css")
	assert.NotContains(t, tpl, "pico.min.css")
}

func TestFixesAlwaysLoaded(t *testing.T) {
	tpl := New().cssTemplate()
	assert.Contains(t, tpl, "pico/pico-fixes.min.css")
	assert.Greater(t, strings.Index(tpl, "pico/pico-fixes.min.css"),
		strings.Index(tpl, "pico/pico.min.css"),
		"fixes must follow the main stylesheet so source-order cascade applies")
}

func TestFixesIgnoredWithCustomCSS(t *testing.T) {
	tpl := New(WithCustomCSS("myapp/custom.css")).cssTemplate()
	assert.Contains(t, tpl, "myapp/custom.css")
	assert.NotContains(t, tpl, "pico-fixes.min.css")
}

func TestStaticFSContainsCompactAndFixes(t *testing.T) {
	_, fsys := New().StaticFS()
	for _, name := range []string{"pico-compact.min.css", "pico-fixes.min.css"} {
		f, err := fsys.Open(name)
		require.NoError(t, err, "expected %s to exist in static FS", name)
		_ = f.Close()
	}
}

func TestMiddlewareInjectsLayout(t *testing.T) {
	mws := New().Middleware()
	require.Len(t, mws, 1)

	var got string
	handler := mws[0](http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = burrow.Layout(r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), newGetRequest())
	assert.Equal(t, "pico/layout", got)
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
	assert.Equal(t, "pico/layout", Layout())
}

func TestNavLayoutReturnsTemplateName(t *testing.T) {
	assert.Equal(t, "pico/nav_layout", NavLayout())
}

func TestOverlayFS_OpenCSSHTML(t *testing.T) {
	ofs := &overlayFS{
		base:    nil, // not needed for css.html
		cssHTML: `<link rel="stylesheet" href="/static/pico/custom.css">`,
	}

	f, err := ofs.Open("css.html")
	require.NoError(t, err)
	defer f.Close()

	data := make([]byte, 200)
	n, _ := f.Read(data)
	assert.Contains(t, string(data[:n]), "custom.css")

	info, err := f.Stat()
	require.NoError(t, err)
	assert.Equal(t, "css.html", info.Name())
	assert.False(t, info.IsDir())
	assert.Equal(t, int64(len(ofs.cssHTML)), info.Size())
	assert.Equal(t, fs.FileMode(0o444), info.Mode())
	assert.NotNil(t, info.ModTime())
	assert.Nil(t, info.Sys())
}

func TestAllColorsNonEmpty(t *testing.T) {
	colors := AllColors()
	assert.Len(t, colors, 20) // Default + 19 named accents
	assert.Contains(t, colors, Default)
	assert.Contains(t, colors, Blue)
	assert.Contains(t, colors, Zinc)
}
