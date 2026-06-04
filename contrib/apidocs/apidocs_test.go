package apidocs

import (
	"bytes"
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ burrow.App             = (*App)(nil)
	_ burrow.HasStaticFiles  = (*App)(nil)
	_ burrow.HasTemplates    = (*App)(nil)
	_ burrow.HasRoutes       = (*App)(nil)
	_ burrow.HasDependencies = (*App)(nil)
)

func TestAppName(t *testing.T) {
	assert.Equal(t, "apidocs", New().Name())
}

func TestDependencies(t *testing.T) {
	assert.Equal(t, []string{"staticfiles"}, New().Dependencies())
}

func TestStaticFS(t *testing.T) {
	prefix, fsys := New().StaticFS()
	assert.Equal(t, "apidocs", prefix)
	require.NotNil(t, fsys)

	f, err := fsys.Open("scalar.standalone.js")
	require.NoError(t, err, "expected vendored Scalar bundle in static FS")
	_ = f.Close()
}

func TestTemplateFS(t *testing.T) {
	fsys := New().TemplateFS()
	require.NotNil(t, fsys)

	f, err := fsys.Open("apidocs.html")
	require.NoError(t, err, "expected apidocs.html in template FS")
	_ = f.Close()
}

func TestOptionsDefaults(t *testing.T) {
	a := New()
	assert.Equal(t, "/api/openapi.json", a.specURL)
	assert.Equal(t, "/api/docs", a.route)
	assert.Equal(t, "API Documentation", a.title)
}

func TestOptionsOverride(t *testing.T) {
	a := New(
		WithSpecURL("/v2/spec.json"),
		WithRoute("/docs"),
		WithTitle("My API"),
	)
	assert.Equal(t, "/v2/spec.json", a.specURL)
	assert.Equal(t, "/docs", a.route)
	assert.Equal(t, "My API", a.title)
}

// testExecutor parses the contrib's real templates with a stubbed staticURL
// func, mirroring how the staticfiles funcmap resolves a hashed asset URL at
// runtime. It lets the page render without booting a full server.
func testExecutor(t *testing.T) burrow.TemplateExecutor {
	t.Helper()
	funcMap := template.FuncMap{
		"staticURL": func(name string) string { return "/static/hashed-" + name },
	}
	tmpl := template.Must(template.New("").Funcs(funcMap).Parse(``))
	_, err := tmpl.ParseFS(New().TemplateFS(), "*.html")
	require.NoError(t, err)

	return func(_ context.Context, name string, data map[string]any) (template.HTML, error) {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			return "", err
		}
		return template.HTML(buf.String()), nil //nolint:gosec // test
	}
}

func TestServePage(t *testing.T) {
	app := New(
		WithSpecURL("/api/openapi.json"),
		WithRoute("/api/docs"),
		WithTitle("Notes API"),
	)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := burrow.WithTemplateExecutor(req.Context(), testExecutor(t))
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	app.Routes(r)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/docs", nil)
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "/api/openapi.json", "spec URL must be wired into the page")
	assert.Contains(t, body, "scalar.standalone.js", "vendored Scalar bundle must be referenced")
	assert.Contains(t, body, "Notes API", "title must be rendered")
	assert.Contains(t, body, "createApiReference", "Scalar init must be present")
}
