package i18n

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testTranslationsFS = fstest.MapFS{
	"translations/active.en.toml": &fstest.MapFile{
		Data: []byte("hello = \"Hello\"\ngreeting = \"Hello, {{.Name}}!\"\n\n[items_count]\none = \"{{.Count}} item\"\nother = \"{{.Count}} items\"\n"),
	},
	"translations/active.de.toml": &fstest.MapFile{
		Data: []byte("hello = \"Hallo\"\ngreeting = \"Hallo, {{.Name}}!\"\n\n[items_count]\none = \"{{.Count}} Artikel\"\nother = \"{{.Count}} Artikel\"\n"),
	},
}

func testBundle(t *testing.T) *Bundle {
	t.Helper()
	b, err := NewTestBundle("en", testTranslationsFS)
	require.NoError(t, err)
	return b
}

func TestNewBundleCreatesBundle(t *testing.T) {
	b, err := NewBundle("en", []string{"en", "de"})
	require.NoError(t, err)
	require.NotNil(t, b.bundle)
}

func TestAddTranslations(t *testing.T) {
	b, err := NewBundle("en", []string{"en", "de"})
	require.NoError(t, err)
	require.NoError(t, b.AddTranslations(testTranslationsFS))

	ctx := b.WithLocale(context.Background(), "en")
	assert.Equal(t, "Hello", T(ctx, "hello"))
}

func TestWithLocale(t *testing.T) {
	b := testBundle(t)

	ctx := b.WithLocale(context.Background(), "de")
	assert.Equal(t, "de", Locale(ctx))
	assert.Equal(t, "Hallo", T(ctx, "hello"))
}

func TestWithLocaleFallsBackToDefault(t *testing.T) {
	b := testBundle(t)

	// "fr" is not supported, should fall back to "en".
	ctx := b.WithLocale(context.Background(), "fr")
	assert.Equal(t, "en", Locale(ctx))
	assert.Equal(t, "Hello", T(ctx, "hello"))
}

func TestMiddlewareSetsLocale(t *testing.T) {
	b := testBundle(t)

	r := chi.NewRouter()
	r.Use(b.LocaleMiddleware())

	var gotLocale string
	var gotTranslation string
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		gotLocale = Locale(ctx)
		gotTranslation = T(ctx, "hello")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9,en;q=0.8")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "de", gotLocale)
	assert.Equal(t, "Hallo", gotTranslation)
}

func TestMiddlewareDefaultsToEnglish(t *testing.T) {
	b := testBundle(t)

	r := chi.NewRouter()
	r.Use(b.LocaleMiddleware())

	var gotLocale string
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		gotLocale = Locale(r.Context())
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "en", gotLocale)
}

func TestRequestFuncMap(t *testing.T) {
	b := testBundle(t)
	ctx := b.WithLocale(context.Background(), "de")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req = req.WithContext(ctx)

	fm := b.RequestFuncMap(req)

	tFunc := fm["t"].(func(string) string)
	assert.Equal(t, "Hallo", tFunc("hello"))

	tDataFunc := fm["tData"].(func(string, map[string]any) string)
	assert.Equal(t, "Hallo, World!", tDataFunc("greeting", map[string]any{"Name": "World"}))

	tPluralFunc := fm["tPlural"].(func(string, int) string)
	assert.Equal(t, "1 Artikel", tPluralFunc("items_count", 1))
	assert.Equal(t, "5 Artikel", tPluralFunc("items_count", 5))
}

func TestBuiltinValidationTranslationsEnglish(t *testing.T) {
	b, err := NewTestBundle("en")
	require.NoError(t, err)

	ctx := b.WithLocale(context.Background(), "en")
	got := TData(ctx, "validation-required", map[string]any{"Field": "Email", "Param": ""})
	assert.Equal(t, "Email is required", got)
}

func TestBuiltinValidationTranslationsGerman(t *testing.T) {
	b, err := NewTestBundle("en")
	require.NoError(t, err)

	ctx := b.WithLocale(context.Background(), "de")
	got := TData(ctx, "validation-min", map[string]any{"Field": "Name", "Param": "3"})
	assert.Equal(t, "Name muss mindestens 3 sein", got)
}

func TestAutoDiscoverTranslations(t *testing.T) {
	b, err := NewBundle("en", []string{"en", "de"})
	require.NoError(t, err)

	appFS := fstest.MapFS{
		"translations/active.en.toml": &fstest.MapFile{
			Data: []byte("auto-key = \"Auto Value\"\n"),
		},
		"translations/active.de.toml": &fstest.MapFile{
			Data: []byte("auto-key = \"Auto Wert\"\n"),
		},
	}
	require.NoError(t, b.AddTranslations(appFS))

	ctx := b.WithLocale(context.Background(), "en")
	assert.Equal(t, "Auto Value", T(ctx, "auto-key"))

	ctx = b.WithLocale(context.Background(), "de")
	assert.Equal(t, "Auto Wert", T(ctx, "auto-key"))
}

func TestNewTestBundle(t *testing.T) {
	b, err := NewTestBundle("en", testTranslationsFS)
	require.NoError(t, err)
	require.NotNil(t, b)

	ctx := b.WithLocale(context.Background(), "en")
	assert.Equal(t, "Hello", T(ctx, "hello"))
}

func TestAppOverrideTranslations(t *testing.T) {
	overrideFS := fstest.MapFS{
		"translations/active.de.toml": &fstest.MapFile{
			Data: []byte("validation-required = \"{{.Field}} wird benötigt\"\n"),
		},
	}

	b, err := NewTestBundle("en", overrideFS)
	require.NoError(t, err)

	ctx := b.WithLocale(context.Background(), "de")
	got := TData(ctx, "validation-required", map[string]any{"Field": "Name", "Param": ""})
	assert.Equal(t, "Name wird benötigt", got)
}

func TestExampleLocale(t *testing.T) {
	// Without a locale in context, Locale defaults to "en".
	assert.Equal(t, "en", Locale(context.Background()))
}

func TestExampleTFallback(t *testing.T) {
	// Without a localizer, T returns the message key as-is.
	assert.Equal(t, "greeting.hello", T(context.Background(), "greeting.hello"))
}

func TestRequestFuncMapLang(t *testing.T) {
	b := testBundle(t)
	ctx := b.WithLocale(context.Background(), "de")
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)

	fm := b.RequestFuncMap(req)
	langFunc := fm["lang"].(func() string)
	assert.Equal(t, "de", langFunc())
}

func TestAddTranslationsInvalidFile(t *testing.T) {
	b, err := NewBundle("en", []string{"en"})
	require.NoError(t, err)

	// A file with invalid TOML content should cause LoadMessageFileFS to fail.
	badFS := fstest.MapFS{
		"translations/active.en.toml": &fstest.MapFile{
			Data: []byte("= invalid toml [[["),
		},
	}
	err = b.AddTranslations(badFS)
	assert.Error(t, err)
}

func TestNewTestBundleWithInvalidTranslations(t *testing.T) {
	badFS := fstest.MapFS{
		"translations/active.en.toml": &fstest.MapFile{
			Data: []byte("= invalid toml [[["),
		},
	}
	_, err := NewTestBundle("en", badFS)
	assert.Error(t, err)
}

func TestNewBundleSkipsEmptyAndDuplicateLanguages(t *testing.T) {
	// Empty strings and duplicates of the default language should be silently ignored.
	b, err := NewBundle("en", []string{"en", "", "  ", "en", "de"})
	require.NoError(t, err)
	require.NotNil(t, b)

	// German should still work.
	ctx := b.WithLocale(context.Background(), "de")
	assert.Equal(t, "de", Locale(ctx))
}
