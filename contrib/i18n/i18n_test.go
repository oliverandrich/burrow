package i18n

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"codeberg.org/oliverandrich/go-webapp-template/core"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// Compile-time interface assertions.
var (
	_ core.App           = (*App)(nil)
	_ core.Configurable  = (*App)(nil)
	_ core.HasMiddleware = (*App)(nil)
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
	app := &App{}
	assert.Equal(t, "i18n", app.Name())
}

func TestAppFlags(t *testing.T) {
	app := &App{}
	flags := app.Flags()

	names := make(map[string]bool)
	for _, f := range flags {
		names[f.Names()[0]] = true
	}

	assert.True(t, names["i18n-default-language"])
	assert.True(t, names["i18n-supported-languages"])
}

func configuredApp(t *testing.T) *App {
	t.Helper()
	app := &App{}
	_ = app.Register(&core.AppConfig{})

	cmd := &cli.Command{
		Name:  "test",
		Flags: app.Flags(),
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

func TestT(t *testing.T) {
	app := configuredApp(t)
	err := app.AddTranslations(testTranslationsFS)
	require.NoError(t, err)

	ctx := app.WithLocale(context.Background(), "en")
	assert.Equal(t, "Hello", T(ctx, "hello"))
}

func TestTGerman(t *testing.T) {
	app := configuredApp(t)
	err := app.AddTranslations(testTranslationsFS)
	require.NoError(t, err)

	ctx := app.WithLocale(context.Background(), "de")
	assert.Equal(t, "Hallo", T(ctx, "hello"))
}

func TestTFallsBackToKey(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "nonexistent_key", T(ctx, "nonexistent_key"))
}

func TestTData(t *testing.T) {
	app := configuredApp(t)
	err := app.AddTranslations(testTranslationsFS)
	require.NoError(t, err)

	ctx := app.WithLocale(context.Background(), "en")
	assert.Equal(t, "Hello, World!", TData(ctx, "greeting", map[string]any{"Name": "World"}))
}

func TestTDataGerman(t *testing.T) {
	app := configuredApp(t)
	err := app.AddTranslations(testTranslationsFS)
	require.NoError(t, err)

	ctx := app.WithLocale(context.Background(), "de")
	assert.Equal(t, "Hallo, World!", TData(ctx, "greeting", map[string]any{"Name": "World"}))
}

func TestTDataFallsBackToKey(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "greeting", TData(ctx, "greeting", map[string]any{"Name": "World"}))
}

func TestTPlural(t *testing.T) {
	app := configuredApp(t)
	err := app.AddTranslations(testTranslationsFS)
	require.NoError(t, err)

	ctx := app.WithLocale(context.Background(), "en")
	assert.Equal(t, "1 item", TPlural(ctx, "items_count", 1))
	assert.Equal(t, "5 items", TPlural(ctx, "items_count", 5))
}

func TestTPluralGerman(t *testing.T) {
	app := configuredApp(t)
	err := app.AddTranslations(testTranslationsFS)
	require.NoError(t, err)

	ctx := app.WithLocale(context.Background(), "de")
	assert.Equal(t, "1 Artikel", TPlural(ctx, "items_count", 1))
	assert.Equal(t, "5 Artikel", TPlural(ctx, "items_count", 5))
}

func TestTPluralFallsBackToKey(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "items_count", TPlural(ctx, "items_count", 3))
}

func TestLocaleDefault(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "en", Locale(ctx))
}

func TestLocaleFromContext(t *testing.T) {
	app := configuredApp(t)
	ctx := app.WithLocale(context.Background(), "de")
	assert.Equal(t, "de", Locale(ctx))
}

func TestMiddlewareSetsLocale(t *testing.T) {
	app := configuredApp(t)
	err := app.AddTranslations(testTranslationsFS)
	require.NoError(t, err)

	e := echo.New()
	for _, mw := range app.Middleware() {
		e.Use(mw)
	}

	var gotLocale string
	var gotTranslation string
	e.GET("/test", func(c *echo.Context) error {
		ctx := c.Request().Context()
		gotLocale = Locale(ctx)
		gotTranslation = T(ctx, "hello")
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9,en;q=0.8")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "de", gotLocale)
	assert.Equal(t, "Hallo", gotTranslation)
}

func TestMiddlewareDefaultsToEnglish(t *testing.T) {
	app := configuredApp(t)
	err := app.AddTranslations(testTranslationsFS)
	require.NoError(t, err)

	e := echo.New()
	for _, mw := range app.Middleware() {
		e.Use(mw)
	}

	var gotLocale string
	e.GET("/test", func(c *echo.Context) error {
		gotLocale = Locale(c.Request().Context())
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "en", gotLocale)
}
