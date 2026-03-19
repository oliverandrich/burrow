package burrow

import (
	"context"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// integrationApp implements HasRoutes, HasTemplates, HasFuncMap, and HasNavItems.
type integrationApp struct{}

func (a *integrationApp) Name() string                { return "integration" }
func (a *integrationApp) Register(_ *AppConfig) error { return nil }

func (a *integrationApp) TemplateFS() fs.FS {
	return fstest.MapFS{
		"layout.html": &fstest.MapFile{
			Data: []byte(`{{ define "integration/layout" }}<html><nav>{{ range navLinks }}<a href="{{ .URL }}">{{ .Label }}</a>{{ end }}</nav><main>{{ .Content }}</main></html>{{ end }}`),
		},
		"home.html": &fstest.MapFile{
			Data: []byte(`{{ define "integration/home" }}<h1>{{ shout .Greeting }}</h1>{{ end }}`),
		},
		"fragment.html": &fstest.MapFile{
			Data: []byte(`{{ define "integration/fragment" }}<p>fragment content</p>{{ end }}`),
		},
	}
}

func (a *integrationApp) FuncMap() template.FuncMap {
	return template.FuncMap{
		"shout": func(s string) string { return strings.ToUpper(s) },
	}
}

func (a *integrationApp) NavItems() []NavItem {
	return []NavItem{
		{Label: "Home", URL: "/", Position: 1},
		{Label: "About", URL: "/about", Position: 2},
	}
}

func (a *integrationApp) Routes(r chi.Router) {
	r.Get("/", Handle(func(w http.ResponseWriter, r *http.Request) error {
		return Render(w, r, http.StatusOK, "integration/home", map[string]any{
			"Greeting": "hello world",
		})
	}))
	r.Get("/fragment", Handle(func(w http.ResponseWriter, r *http.Request) error {
		return Render(w, r, http.StatusOK, "integration/fragment", nil)
	}))
}

// buildIntegrationRouter creates a fully bootstrapped chi router
// mirroring the Server.Run() boot sequence, but without starting
// an HTTP listener.
func buildIntegrationRouter(t *testing.T) chi.Router {
	t.Helper()

	app := &integrationApp{}
	srv := NewServer(app)
	srv.SetLayout("integration/layout")

	// Open an in-memory database, matching the real boot sequence.
	db, err := openDB(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// Create i18n bundle (required by the boot sequence).
	bundle, err := i18n.NewBundle("en", []string{"en"})
	require.NoError(t, err)
	srv.i18nBundle = bundle

	// Bootstrap: migrations, Register, seed.
	cfg := &Config{
		Server:   ServerConfig{Host: "localhost", Port: 8080, BaseURL: "http://localhost:8080"},
		Database: DatabaseConfig{DSN: ":memory:"},
		I18n:     I18nConfig{DefaultLanguage: "en", SupportedLanguages: "en"},
	}
	ctx := context.Background()
	err = srv.bootstrap(ctx, db, cfg)
	require.NoError(t, err)

	// Register core request func map providers (mirrors Run()).
	srv.requestFuncMapProviders = append(srv.requestFuncMapProviders, srv.i18nBundle.RequestFuncMap)
	srv.requestFuncMapProviders = append(srv.requestFuncMapProviders, coreRequestFuncMap)

	// Build templates.
	err = srv.buildTemplates()
	require.NoError(t, err)

	// Build router with middleware (mirrors Run()).
	r := chi.NewRouter()
	r.Use(srv.i18nBundle.LocaleMiddleware())
	navItems := srv.registry.AllNavItems()
	r.Use(navItemsMiddleware(navItems))
	r.Use(layoutMiddleware(srv.layout))
	r.Use(srv.templateMiddleware())
	srv.registry.RegisterMiddleware(r)
	srv.registry.RegisterRoutes(r)

	r.NotFound(Handle(func(w http.ResponseWriter, r *http.Request) error {
		return NewHTTPError(http.StatusNotFound, "page not found")
	}))
	r.MethodNotAllowed(Handle(func(w http.ResponseWriter, r *http.Request) error {
		return NewHTTPError(http.StatusMethodNotAllowed, "method not allowed")
	}))

	return r
}

func TestIntegration_TemplateRenderingWithFuncMap(t *testing.T) {
	router := buildIntegrationRouter(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	// The custom FuncMap "shout" should uppercase the greeting.
	assert.Contains(t, body, "HELLO WORLD")
}

func TestIntegration_NavLinksInResponse(t *testing.T) {
	router := buildIntegrationRouter(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	// Nav items should be rendered via the navLinks template function.
	assert.Contains(t, body, `<a href="/">Home</a>`)
	assert.Contains(t, body, `<a href="/about">About</a>`)
}

func TestIntegration_LayoutWrapping(t *testing.T) {
	router := buildIntegrationRouter(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	// Layout should wrap the content in <html>...<main>...</main>...</html>.
	assert.Contains(t, body, "<html>")
	assert.Contains(t, body, "<nav>")
	assert.Contains(t, body, "<main>")
	assert.Contains(t, body, "</html>")

	// Content should be inside <main>.
	mainIdx := strings.Index(body, "<main>")
	endMainIdx := strings.Index(body, "</main>")
	require.Positive(t, mainIdx)
	require.Greater(t, endMainIdx, mainIdx)
	content := body[mainIdx:endMainIdx]
	assert.Contains(t, content, "HELLO WORLD")
}

func TestIntegration_HTMXRequestReturnsFragmentOnly(t *testing.T) {
	router := buildIntegrationRouter(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/fragment", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	// Fragment only: no layout wrapping.
	assert.Contains(t, body, "<p>fragment content</p>")
	assert.NotContains(t, body, "<html>")
	assert.NotContains(t, body, "<nav>")
	assert.NotContains(t, body, "<main>")
}

func TestIntegration_404ErrorPage(t *testing.T) {
	router := buildIntegrationRouter(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	body := rec.Body.String()
	assert.Contains(t, body, "404")
	assert.Contains(t, body, "The page you are looking for does not exist.")

	// Error pages bypass the app layout — no nav wrapping.
	assert.NotContains(t, body, "<nav>")
}

func TestIntegration_404JSONForAPI(t *testing.T) {
	router := buildIntegrationRouter(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/nonexistent", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "page not found", body["error"])
}

func TestIntegration_405ErrorPage(t *testing.T) {
	router := buildIntegrationRouter(t)

	// GET / is defined, but DELETE / is not.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "405")

	// Error pages bypass the app layout — no nav wrapping.
	assert.NotContains(t, body, "<nav>")
}

func TestIntegration_ErrorPageHasTitleField(t *testing.T) {
	router := buildIntegrationRouter(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	// The i18n bundle has the error-404-title key loaded, so the localized title
	// "Not Found" should appear in the rendered output.
	assert.Contains(t, rec.Body.String(), "Not Found")
}
