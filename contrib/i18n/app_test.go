package i18n

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// Compile-time interface assertions.
var (
	_ burrow.App               = (*App)(nil)
	_ burrow.Configurable      = (*App)(nil)
	_ burrow.HasMiddleware     = (*App)(nil)
	_ burrow.HasRequestFuncMap = (*App)(nil)
)

var testTranslationsFS = fstest.MapFS{
	"translations/active.en.toml": &fstest.MapFile{
		Data: []byte("hello = \"Hello\"\ngreeting = \"Hello, {{.Name}}!\"\n\n[items_count]\none = \"{{.Count}} item\"\nother = \"{{.Count}} items\"\n"),
	},
	"translations/active.de.toml": &fstest.MapFile{
		Data: []byte("hello = \"Hallo\"\ngreeting = \"Hallo, {{.Name}}!\"\n\n[items_count]\none = \"{{.Count}} Artikel\"\nother = \"{{.Count}} Artikel\"\n"),
	},
}

func TestAppName(t *testing.T) {
	app := New()
	assert.Equal(t, "i18n", app.Name())
}

func TestAppFlags(t *testing.T) {
	app := New()
	flags := app.Flags(nil)

	names := make(map[string]bool)
	for _, f := range flags {
		names[f.Names()[0]] = true
	}

	assert.True(t, names["i18n-default-language"])
	assert.True(t, names["i18n-supported-languages"])
}

func configuredApp(t *testing.T) *App {
	t.Helper()
	app := New()
	_ = app.Register(&burrow.AppConfig{})

	cmd := &cli.Command{
		Name:  "test",
		Flags: app.Flags(nil),
		Action: func(_ context.Context, cmd *cli.Command) error {
			return app.Configure(cmd)
		},
	}
	err := cmd.Run(t.Context(), []string{"test"})
	require.NoError(t, err)
	return app
}

func TestConfigureCreatesBundle(t *testing.T) {
	app := configuredApp(t)
	require.NotNil(t, app.bundle)
}

func TestMiddlewareSetsLocale(t *testing.T) {
	app := configuredApp(t)
	err := app.AddTranslations(testTranslationsFS)
	require.NoError(t, err)

	r := chi.NewRouter()
	for _, mw := range app.Middleware() {
		r.Use(mw)
	}

	var gotLocale string
	var gotTranslation string
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		gotLocale = Locale(ctx)
		gotTranslation = T(ctx, "hello")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9,en;q=0.8")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "de", gotLocale)
	assert.Equal(t, "Hallo", gotTranslation)
}

// mockTranslationApp is a mock app implementing HasTranslations.
type mockTranslationApp struct {
	fs fstest.MapFS
}

func (m *mockTranslationApp) Name() string                       { return "mock" }
func (m *mockTranslationApp) Register(_ *burrow.AppConfig) error { return nil }
func (m *mockTranslationApp) TranslationFS() fs.FS               { return m.fs }

func TestAutoDiscoverTranslations(t *testing.T) {
	mock := &mockTranslationApp{
		fs: fstest.MapFS{
			"translations/active.en.toml": &fstest.MapFile{
				Data: []byte("auto-key = \"Auto Value\"\n"),
			},
			"translations/active.de.toml": &fstest.MapFile{
				Data: []byte("auto-key = \"Auto Wert\"\n"),
			},
		},
	}

	registry := burrow.NewRegistry()
	registry.Add(mock)

	app := New()
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	cmd := &cli.Command{
		Name:  "test",
		Flags: app.Flags(nil),
		Action: func(_ context.Context, cmd *cli.Command) error {
			return app.Configure(cmd)
		},
	}
	err := cmd.Run(t.Context(), []string{"test"})
	require.NoError(t, err)

	ctx := app.WithLocale(context.Background(), "en")
	assert.Equal(t, "Auto Value", T(ctx, "auto-key"))

	ctx = app.WithLocale(context.Background(), "de")
	assert.Equal(t, "Auto Wert", T(ctx, "auto-key"))
}

func TestAutoDiscoverSkipsAppsWithoutTranslations(t *testing.T) {
	// An app that does not implement HasTranslations.
	plainApp := New()

	registry := burrow.NewRegistry()
	registry.Add(plainApp)

	app := New()
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	cmd := &cli.Command{
		Name:  "test",
		Flags: app.Flags(nil),
		Action: func(_ context.Context, cmd *cli.Command) error {
			return app.Configure(cmd)
		},
	}
	err := cmd.Run(t.Context(), []string{"test"})
	require.NoError(t, err)

	// Should not panic or error — just no extra translations loaded.
	ctx := app.WithLocale(context.Background(), "en")
	assert.Equal(t, "nonexistent", T(ctx, "nonexistent"))
}

func TestBuiltinValidationTranslationsEnglish(t *testing.T) {
	app := configuredApp(t)

	ctx := app.WithLocale(context.Background(), "en")
	got := TData(ctx, "validation-required", map[string]any{"Field": "Email", "Param": ""})
	assert.Equal(t, "Email is required", got)
}

func TestBuiltinValidationTranslationsGerman(t *testing.T) {
	app := configuredApp(t)

	ctx := app.WithLocale(context.Background(), "de")
	got := TData(ctx, "validation-min", map[string]any{"Field": "Name", "Param": "3"})
	assert.Equal(t, "Name muss mindestens 3 sein", got)
}

func TestRequestFuncMap(t *testing.T) {
	app := configuredApp(t)
	require.NoError(t, app.AddTranslations(testTranslationsFS))

	ctx := app.WithLocale(context.Background(), "de")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req = req.WithContext(ctx)

	fm := app.RequestFuncMap(req)

	tFunc := fm["t"].(func(string) string)
	assert.Equal(t, "Hallo", tFunc("hello"))

	tDataFunc := fm["tData"].(func(string, map[string]any) string)
	assert.Equal(t, "Hallo, World!", tDataFunc("greeting", map[string]any{"Name": "World"}))

	tPluralFunc := fm["tPlural"].(func(string, int) string)
	assert.Equal(t, "1 Artikel", tPluralFunc("items_count", 1))
	assert.Equal(t, "5 Artikel", tPluralFunc("items_count", 5))
}

func TestMiddlewareDefaultsToEnglish(t *testing.T) {
	app := configuredApp(t)
	err := app.AddTranslations(testTranslationsFS)
	require.NoError(t, err)

	r := chi.NewRouter()
	for _, mw := range app.Middleware() {
		r.Use(mw)
	}

	var gotLocale string
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		gotLocale = Locale(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "en", gotLocale)
}
