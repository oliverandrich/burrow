package i18n

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
