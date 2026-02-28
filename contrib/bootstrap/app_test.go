package bootstrap

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	bstpl "codeberg.org/oliverandrich/burrow/contrib/bootstrap/templates"
	"codeberg.org/oliverandrich/burrow/contrib/messages"
	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ burrow.App            = (*App)(nil)
	_ burrow.HasStaticFiles = (*App)(nil)
	_ burrow.HasMiddleware  = (*App)(nil)
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

func TestStaticFS(t *testing.T) {
	app := New()
	prefix, fsys := app.StaticFS()

	assert.Equal(t, "bootstrap", prefix)
	require.NotNil(t, fsys)

	for _, name := range []string{
		"bootstrap.min.css",
		"bootstrap.bundle.min.js",
		"bootstrap-icons.min.css",
		"htmx.min.js",
		"fonts/bootstrap-icons.woff2",
		"fonts/bootstrap-icons.woff",
	} {
		f, err := fsys.Open(name)
		require.NoError(t, err, "expected %s to exist in static FS", name)
		_ = f.Close()
	}
}

func TestLayout(t *testing.T) {
	layout := Layout()
	require.NotNil(t, layout)

	content := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, "<p>test content</p>")
		return err
	})

	component := layout("Test Page", content)
	require.NotNil(t, component)

	var buf strings.Builder
	err := component.Render(context.Background(), &buf)
	require.NoError(t, err)

	html := buf.String()
	assert.Contains(t, html, "<!doctype html>")
	assert.Contains(t, html, "<title>Test Page</title>")
	assert.Contains(t, html, "bootstrap.min.css")
	assert.Contains(t, html, "bootstrap-icons.min.css")
	assert.Contains(t, html, "bootstrap.bundle.min.js")
	assert.Contains(t, html, "htmx.min.js")
	assert.Contains(t, html, "<p>test content</p>")
}

func TestMiddlewareInjectsLayout(t *testing.T) {
	app := New()
	mws := app.Middleware()
	require.Len(t, mws, 1)

	var got burrow.LayoutFunc
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = burrow.Layout(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := mws[0](inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotNil(t, got, "middleware should inject layout when none is set")
}

func TestAlertsEmpty(t *testing.T) {
	var buf strings.Builder
	err := bstpl.Alerts().Render(context.Background(), &buf)
	require.NoError(t, err)

	html := buf.String()
	assert.Contains(t, html, `<div id="alerts">`)
	assert.NotContains(t, html, "alert-dismissible")
}

func TestAlertsRendersMessages(t *testing.T) {
	ctx := messages.Inject(context.Background(), []messages.Message{
		{Level: messages.Success, Text: "Note created"},
		{Level: messages.Error, Text: "Something failed"},
	})

	var buf strings.Builder
	err := bstpl.Alerts().Render(ctx, &buf)
	require.NoError(t, err)

	html := buf.String()
	assert.Contains(t, html, `<div id="alerts">`)
	assert.NotContains(t, html, "hx-swap-oob")
	assert.Contains(t, html, "alert-success")
	assert.Contains(t, html, "Note created")
	assert.Contains(t, html, "alert-danger")
	assert.Contains(t, html, "Something failed")
	assert.Contains(t, html, "alert-dismissible")
	assert.Contains(t, html, "btn-close")
}

func TestAlertsOOBEmpty(t *testing.T) {
	var buf strings.Builder
	err := bstpl.AlertsOOB().Render(context.Background(), &buf)
	require.NoError(t, err)

	html := buf.String()
	assert.Contains(t, html, `<div id="alerts"`)
	assert.Contains(t, html, `hx-swap-oob="true"`)
	assert.NotContains(t, html, "alert-dismissible")
}

func TestAlertsOOBRendersMessages(t *testing.T) {
	ctx := messages.Inject(context.Background(), []messages.Message{
		{Level: messages.Success, Text: "Note created"},
		{Level: messages.Error, Text: "Something failed"},
	})

	var buf strings.Builder
	err := bstpl.AlertsOOB().Render(ctx, &buf)
	require.NoError(t, err)

	html := buf.String()
	assert.Contains(t, html, `<div id="alerts"`)
	assert.Contains(t, html, `hx-swap-oob="true"`)
	assert.Contains(t, html, "alert-success")
	assert.Contains(t, html, "Note created")
	assert.Contains(t, html, "alert-danger")
	assert.Contains(t, html, "Something failed")
	assert.Contains(t, html, "alert-dismissible")
	assert.Contains(t, html, "btn-close")
}

func TestMiddlewareDoesNotOverride(t *testing.T) {
	app := New()
	mws := app.Middleware()
	require.Len(t, mws, 1)

	existing := burrow.LayoutFunc(func(_ string, content templ.Component) templ.Component {
		return content
	})

	var got burrow.LayoutFunc
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = burrow.Layout(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := mws[0](inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Pre-set a layout in context.
	req = req.WithContext(burrow.WithLayout(req.Context(), existing))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotNil(t, got, "layout should still be set")
	// Verify it's the original, not bootstrap's — use a known sentinel.
	result := got("x", templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, "sentinel")
		return err
	}))
	var buf strings.Builder
	require.NoError(t, result.Render(context.Background(), &buf))
	assert.Equal(t, "sentinel", buf.String(), "middleware should not overwrite existing layout")
}
