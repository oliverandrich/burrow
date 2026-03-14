package i18n

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestT(t *testing.T) {
	bundle := testBundle(t)
	ctx := bundle.WithLocale(context.Background(), "en")
	assert.Equal(t, "Hello", T(ctx, "hello"))
}

func TestTGerman(t *testing.T) {
	bundle := testBundle(t)
	ctx := bundle.WithLocale(context.Background(), "de")
	assert.Equal(t, "Hallo", T(ctx, "hello"))
}

func TestTFallsBackToKey(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "nonexistent_key", T(ctx, "nonexistent_key"))
}

func TestTData(t *testing.T) {
	bundle := testBundle(t)
	ctx := bundle.WithLocale(context.Background(), "en")
	assert.Equal(t, "Hello, World!", TData(ctx, "greeting", map[string]any{"Name": "World"}))
}

func TestTDataGerman(t *testing.T) {
	bundle := testBundle(t)
	ctx := bundle.WithLocale(context.Background(), "de")
	assert.Equal(t, "Hallo, World!", TData(ctx, "greeting", map[string]any{"Name": "World"}))
}

func TestTDataFallsBackToKey(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "greeting", TData(ctx, "greeting", map[string]any{"Name": "World"}))
}

func TestTPlural(t *testing.T) {
	bundle := testBundle(t)
	ctx := bundle.WithLocale(context.Background(), "en")
	assert.Equal(t, "1 item", TPlural(ctx, "items_count", 1))
	assert.Equal(t, "5 items", TPlural(ctx, "items_count", 5))
}

func TestTPluralGerman(t *testing.T) {
	bundle := testBundle(t)
	ctx := bundle.WithLocale(context.Background(), "de")
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
	bundle := testBundle(t)
	ctx := bundle.WithLocale(context.Background(), "de")
	assert.Equal(t, "de", Locale(ctx))
}

func TestValidationTranslateEnglish(t *testing.T) {
	bundle, err := NewTestBundle("en")
	require.NoError(t, err)
	ctx := bundle.WithLocale(context.Background(), "en")

	got := TData(ctx, "validation-required", map[string]any{"Field": "Email", "Param": ""})
	assert.Equal(t, "Email is required", got)
}

func TestValidationTranslateGerman(t *testing.T) {
	bundle, err := NewTestBundle("en")
	require.NoError(t, err)
	ctx := bundle.WithLocale(context.Background(), "de")

	got := TData(ctx, "validation-required", map[string]any{"Field": "Email", "Param": ""})
	assert.Equal(t, "Email ist erforderlich", got)
}

func TestTFallsBackToKeyWhenLocalizerPresent(t *testing.T) {
	bundle := testBundle(t)
	ctx := bundle.WithLocale(context.Background(), "en")
	// Key does not exist in translations — localizer is present but Localize returns an error.
	assert.Equal(t, "nonexistent_key", T(ctx, "nonexistent_key"))
}

func TestTDataFallsBackToKeyWhenLocalizerPresent(t *testing.T) {
	bundle := testBundle(t)
	ctx := bundle.WithLocale(context.Background(), "en")
	// Key does not exist — localizer present but Localize errors.
	assert.Equal(t, "missing_key", TData(ctx, "missing_key", map[string]any{"Name": "test"}))
}

func TestTPluralFallsBackToKeyWhenLocalizerPresent(t *testing.T) {
	bundle := testBundle(t)
	ctx := bundle.WithLocale(context.Background(), "en")
	// Key does not exist — localizer present but Localize errors.
	assert.Equal(t, "missing_plural_key", TPlural(ctx, "missing_plural_key", 3))
}
