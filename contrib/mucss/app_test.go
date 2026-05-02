package mucss

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
	_ burrow.Configurable    = (*App)(nil)
	_ burrow.HasStaticFiles  = (*App)(nil)
	_ burrow.HasMiddleware   = (*App)(nil)
	_ burrow.HasTemplates    = (*App)(nil)
	_ burrow.HasDependencies = (*App)(nil)
)

func TestAppName(t *testing.T) {
	assert.Equal(t, "mucss", New().Name())
}

func TestConfigureRegistersIcons(t *testing.T) {
	cfg := &burrow.AppConfig{}
	require.NoError(t, New().Configure(cfg, nil))

	icons := cfg.IconFuncs()
	assert.Contains(t, icons, "iconSunFill")
	assert.Contains(t, icons, "iconMoonStarsFill")
	assert.Contains(t, icons, "iconCircleHalf")
}

func TestDefaultColor(t *testing.T) {
	assert.Equal(t, Default, New().color)
}

func TestWithColor(t *testing.T) {
	assert.Equal(t, Azure, New(WithColor(Azure)).color)
}

func TestDependencies(t *testing.T) {
	assert.Equal(t, []string{"staticfiles", "htmx"}, New().Dependencies())
}

func TestStaticFS(t *testing.T) {
	prefix, fsys := New().StaticFS()
	assert.Equal(t, "mucss", prefix)
	require.NotNil(t, fsys)

	for _, c := range AllColors() {
		name := "mu.css"
		if c != Default {
			name = "mu." + string(c) + ".css"
		}
		f, err := fsys.Open(name)
		require.NoError(t, err, "expected %s to exist in static FS", name)
		_ = f.Close()
	}

	for _, name := range []string{"mu-compact.min.css", "mu-extras.min.css"} {
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
		"error_default.html",
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
		{Default, "mucss/mu.css"},
		{Azure, "mucss/mu.azure.css"},
		{Purple, "mucss/mu.purple.css"},
		{Zinc, "mucss/mu.zinc.css"},
	}
	for _, tt := range tests {
		app := New(WithColor(tt.color))
		assert.Contains(t, app.cssTemplate(), tt.expected)
	}
}

func TestCSSTemplateCustom(t *testing.T) {
	app := New(WithCustomCSS("myapp/mytheme.css"))
	assert.Contains(t, app.cssTemplate(), "myapp/mytheme.css")
}

func TestWithCustomCSSOverridesColor(t *testing.T) {
	app := New(WithColor(Azure), WithCustomCSS("myapp/custom.css"))
	tpl := app.cssTemplate()
	assert.Contains(t, tpl, "myapp/custom.css")
	assert.NotContains(t, tpl, "mucss/mu.azure.css")
}

func TestWithColorClearsCustomCSS(t *testing.T) {
	app := New(WithCustomCSS("myapp/custom.css"), WithColor(Zinc))
	assert.Contains(t, app.cssTemplate(), "mucss/mu.zinc.css")
}

func TestNoCompactByDefault(t *testing.T) {
	assert.NotContains(t, New().cssTemplate(), "mu-compact.min.css")
}

func TestExtrasAlwaysLoaded(t *testing.T) {
	tpl := New().cssTemplate()
	assert.Contains(t, tpl, "mucss/mu-extras.min.css")
	assert.Greater(t, strings.Index(tpl, "mucss/mu-extras.min.css"),
		strings.Index(tpl, "mucss/mu.css"),
		"extras must follow the main stylesheet so source-order cascade applies")
}

func TestExtrasIgnoredWithCustomCSS(t *testing.T) {
	tpl := New(WithCustomCSS("myapp/custom.css")).cssTemplate()
	assert.Contains(t, tpl, "myapp/custom.css")
	assert.NotContains(t, tpl, "mu-extras.min.css")
}

func TestWithCompactType(t *testing.T) {
	tpl := New(WithCompactType()).cssTemplate()
	assert.Contains(t, tpl, "mucss/mu.css")
	assert.Contains(t, tpl, "mucss/mu-compact.min.css")
	assert.Greater(t, strings.Index(tpl, "mucss/mu-compact.min.css"),
		strings.Index(tpl, "mucss/mu.css"),
		"compact override must follow the main stylesheet so source-order cascade applies")
}

func TestCompactCombinesWithColor(t *testing.T) {
	tpl := New(WithColor(Azure), WithCompactType()).cssTemplate()
	assert.Contains(t, tpl, "mucss/mu.azure.css")
	assert.Contains(t, tpl, "mucss/mu-compact.min.css")
}

func TestCompactIgnoredWithCustomCSS(t *testing.T) {
	tpl := New(WithCustomCSS("myapp/custom.css"), WithCompactType()).cssTemplate()
	assert.Contains(t, tpl, "myapp/custom.css")
	assert.NotContains(t, tpl, "mu-compact.min.css")
	assert.NotContains(t, tpl, "mucss/mu.css")
}

func TestMiddlewareInjectsLayout(t *testing.T) {
	mws := New().Middleware()
	require.Len(t, mws, 1)

	var got string
	handler := mws[0](http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = burrow.Layout(r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), newGetRequest())
	assert.Equal(t, "mucss/layout", got)
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
	assert.Equal(t, "mucss/layout", Layout())
}

func TestNavLayoutReturnsTemplateName(t *testing.T) {
	assert.Equal(t, "mucss/nav_layout", NavLayout())
}

func TestOverlayFS_OpenCSSHTML(t *testing.T) {
	ofs := &overlayFS{
		base:    nil,
		cssHTML: `<link rel="stylesheet" href="/static/mucss/custom.css">`,
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

func TestAllColorsLength(t *testing.T) {
	assert.Len(t, AllColors(), 21) // Default + 20 named accents
}
