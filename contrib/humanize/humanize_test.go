package humanize

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"github.com/oliverandrich/burrow/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCtx(t *testing.T, locale string) context.Context {
	t.Helper()
	b, err := i18n.NewTestBundle(translationFS)
	require.NoError(t, err)
	return b.WithLocale(context.Background(), locale)
}

// --- filesizeformat ---

func TestFilesizeformat(t *testing.T) {
	tests := []struct {
		locale string
		want   string
		bytes  int64
	}{
		{"en", "0 bytes", 0},
		{"en", "1 byte", 1},
		{"en", "100 bytes", 100},
		{"en", "1.0 KB", 1024},
		{"en", "1.5 KB", 1536},
		{"en", "1.0 MB", 1048576},
		{"en", "1.0 GB", 1073741824},
		{"en", "1.0 TB", 1099511627776},
		{"en", "1.0 PB", 1125899906842624},
		{"en", "13.2 KB", 13516},
		// German locale uses comma as decimal separator
		{"de", "1,5 KB", 1536},
		{"de", "13,2 KB", 13516},
		// Negative values
		{"en", "-1.0 KB", -1024},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			ctx := testCtx(t, tt.locale)
			assert.Equal(t, tt.want, filesizeformat(ctx, tt.bytes))
		})
	}
}

// --- intcomma ---

func TestIntcomma(t *testing.T) {
	tests := []struct {
		locale string
		want   string
		n      int
	}{
		{"en", "0", 0},
		{"en", "100", 100},
		{"en", "1,000", 1000},
		{"en", "1,000,000", 1000000},
		{"en", "-1,000", -1000},
		// German uses period as thousands separator
		{"de", "1.000", 1000},
		{"de", "1.000.000", 1000000},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			ctx := testCtx(t, tt.locale)
			assert.Equal(t, tt.want, intcomma(ctx, tt.n))
		})
	}
}

// --- ordinal ---

func TestOrdinal(t *testing.T) {
	tests := []struct {
		locale string
		want   string
		n      int
	}{
		{"en", "1st", 1},
		{"en", "2nd", 2},
		{"en", "3rd", 3},
		{"en", "4th", 4},
		{"en", "11th", 11},
		{"en", "12th", 12},
		{"en", "13th", 13},
		{"en", "21st", 21},
		{"en", "22nd", 22},
		{"en", "23rd", 23},
		{"en", "101st", 101},
		{"en", "111th", 111},
		{"en", "112th", 112},
		// German always uses period
		{"de", "1.", 1},
		{"de", "2.", 2},
		{"de", "21.", 21},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			ctx := testCtx(t, tt.locale)
			assert.Equal(t, tt.want, ordinal(ctx, tt.n))
		})
	}
}

// --- apnumber ---

func TestApnumber(t *testing.T) {
	tests := []struct {
		locale string
		want   string
		n      int
	}{
		{"en", "one", 1},
		{"en", "five", 5},
		{"en", "nine", 9},
		{"en", "10", 10},
		{"en", "0", 0},
		{"en", "-1", -1},
		// German
		{"de", "eins", 1},
		{"de", "fünf", 5},
		{"de", "10", 10},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			ctx := testCtx(t, tt.locale)
			assert.Equal(t, tt.want, apnumber(ctx, tt.n))
		})
	}
}

// --- naturalday ---

func TestNaturalday(t *testing.T) {
	now := time.Date(2026, 4, 1, 15, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		t      time.Time
		locale string
		want   string
	}{
		{"today en", now, "en", "today"},
		{"yesterday en", now.AddDate(0, 0, -1), "en", "yesterday"},
		{"tomorrow en", now.AddDate(0, 0, 1), "en", "tomorrow"},
		{"other en", time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC), "en", "Mar 15, 2026"},
		{"today de", now, "de", "heute"},
		{"yesterday de", now.AddDate(0, 0, -1), "de", "gestern"},
		{"tomorrow de", now.AddDate(0, 0, 1), "de", "morgen"},
		{"other de", time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC), "de", "15.03.2026"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testCtx(t, tt.locale)
			assert.Equal(t, tt.want, naturalday(ctx, "", tt.t, now))
		})
	}
}

func TestNaturaldayCustomFormat(t *testing.T) {
	now := time.Date(2026, 4, 1, 15, 0, 0, 0, time.UTC)
	other := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	ctx := testCtx(t, "en")
	assert.Equal(t, "2026-03-15", naturalday(ctx, "2006-01-02", other, now))
}

// --- naturaltime ---

func TestNaturaltime(t *testing.T) {
	now := time.Date(2026, 4, 1, 15, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		t      time.Time
		locale string
		want   string
	}{
		{"just now", now, "en", "just now"},
		{"seconds ago", now.Add(-30 * time.Second), "en", "30 seconds ago"},
		{"1 minute ago", now.Add(-90 * time.Second), "en", "1 minute ago"},
		{"minutes ago", now.Add(-5 * time.Minute), "en", "5 minutes ago"},
		{"1 hour ago", now.Add(-90 * time.Minute), "en", "1 hour ago"},
		{"hours ago", now.Add(-3 * time.Hour), "en", "3 hours ago"},
		{"1 day ago", now.Add(-36 * time.Hour), "en", "1 day ago"},
		{"days ago", now.Add(-5 * 24 * time.Hour), "en", "5 days ago"},
		{"1 week ago", now.Add(-10 * 24 * time.Hour), "en", "1 week ago"},
		{"weeks ago", now.Add(-20 * 24 * time.Hour), "en", "2 weeks ago"},
		{"1 month ago", now.Add(-45 * 24 * time.Hour), "en", "1 month ago"},
		{"months ago", now.Add(-100 * 24 * time.Hour), "en", "3 months ago"},
		{"1 year ago", now.Add(-400 * 24 * time.Hour), "en", "1 year ago"},
		{"years ago", now.Add(-800 * 24 * time.Hour), "en", "2 years ago"},
		// Future
		{"seconds from now", now.Add(30 * time.Second), "en", "30 seconds from now"},
		{"minutes from now", now.Add(5 * time.Minute), "en", "5 minutes from now"},
		// German
		{"gerade eben", now, "de", "gerade eben"},
		{"vor 5 Minuten", now.Add(-5 * time.Minute), "de", "vor 5 Minuten"},
		{"in 5 Minuten", now.Add(5 * time.Minute), "de", "in 5 Minuten"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testCtx(t, tt.locale)
			assert.Equal(t, tt.want, naturaltime(ctx, tt.t, now))
		})
	}
}

// --- RequestFuncMap integration ---

func TestRequestFuncMapRegistersAllFunctions(t *testing.T) {
	app := New()
	ctx := testCtx(t, "en")
	fm := app.RequestFuncMap(ctx)

	expectedFuncs := []string{"naturaltime", "naturalday", "intcomma", "ordinal", "apnumber", "filesizeformat"}
	for _, name := range expectedFuncs {
		assert.NotNil(t, fm[name], "expected template function %q", name)
	}
}

func TestTranslationFSIsValid(t *testing.T) {
	app := New()
	fs := app.TranslationFS()
	assert.NotNil(t, fs)
}

func TestAppName(t *testing.T) {
	app := New()
	assert.Equal(t, "humanize", app.Name())
}

func TestWithDateFormatOption(t *testing.T) {
	app := New(WithDateFormat("2006-01-02"))
	assert.Equal(t, "2006-01-02", app.dateFormat)
}

// --- Interface assertions ---

func TestAppImplementsInterfaces(t *testing.T) {
	app := New()
	ctx := testCtx(t, "en")

	// HasRequestFuncMap
	fm := app.RequestFuncMap(ctx)
	assert.NotNil(t, fm)

	// HasTranslations
	fs := app.TranslationFS()
	assert.NotNil(t, fs)
}

// --- apnumber without translations (fallback) ---

// --- any-type support (pointers, nil, wrong type) ---

func TestNaturaltimePointer(t *testing.T) {
	ctx := testCtx(t, "en")
	now := time.Now()
	past := now.Add(-5 * time.Minute)
	assert.Equal(t, "5 minutes ago", naturaltime(ctx, &past, now))
}

func TestNaturaltimeNilPointer(t *testing.T) {
	ctx := testCtx(t, "en")
	assert.Empty(t, naturaltime(ctx, (*time.Time)(nil), time.Now()))
}

func TestNaturaltimeWrongType(t *testing.T) {
	ctx := testCtx(t, "en")
	assert.Empty(t, naturaltime(ctx, "not a time", time.Now()))
}

func TestNaturaldayPointer(t *testing.T) {
	ctx := testCtx(t, "en")
	now := time.Date(2026, 4, 1, 15, 0, 0, 0, time.UTC)
	assert.Equal(t, "today", naturalday(ctx, "", &now, now))
}

func TestNaturaldayNilPointer(t *testing.T) {
	ctx := testCtx(t, "en")
	assert.Empty(t, naturalday(ctx, "", (*time.Time)(nil), time.Now()))
}

func TestNaturaldayWrongType(t *testing.T) {
	ctx := testCtx(t, "en")
	assert.Empty(t, naturalday(ctx, "", 12345, time.Now()))
}

func TestIntcommaPointer(t *testing.T) {
	ctx := testCtx(t, "en")
	n := 1000
	assert.Equal(t, "1,000", intcomma(ctx, &n))
}

func TestIntcommaNilPointer(t *testing.T) {
	ctx := testCtx(t, "en")
	assert.Empty(t, intcomma(ctx, (*int)(nil)))
}

func TestIntcommaWrongType(t *testing.T) {
	ctx := testCtx(t, "en")
	assert.Empty(t, intcomma(ctx, "not a number"))
}

func TestOrdinalPointer(t *testing.T) {
	ctx := testCtx(t, "en")
	n := 1
	assert.Equal(t, "1st", ordinal(ctx, &n))
}

func TestOrdinalNilPointer(t *testing.T) {
	ctx := testCtx(t, "en")
	assert.Empty(t, ordinal(ctx, (*int)(nil)))
}

func TestApnumberPointer(t *testing.T) {
	ctx := testCtx(t, "en")
	n := 5
	assert.Equal(t, "five", apnumber(ctx, &n))
}

func TestApnumberNilPointer(t *testing.T) {
	ctx := testCtx(t, "en")
	assert.Empty(t, apnumber(ctx, (*int)(nil)))
}

func TestFilesizeformatPointer(t *testing.T) {
	ctx := testCtx(t, "en")
	n := int64(1024)
	assert.Equal(t, "1.0 KB", filesizeformat(ctx, &n))
}

func TestFilesizeformatNilPointer(t *testing.T) {
	ctx := testCtx(t, "en")
	assert.Empty(t, filesizeformat(ctx, (*int64)(nil)))
}

func TestFilesizeformatWrongType(t *testing.T) {
	ctx := testCtx(t, "en")
	assert.Empty(t, filesizeformat(ctx, "not a number"))
}

func TestApnumberFallbackWithoutLocalizer(t *testing.T) {
	ctx := context.Background()
	// Without a localizer, T falls back to the key — apnumber should return digits.
	assert.Equal(t, "humanize-apnumber-5", apnumber(ctx, 5))
}

// --- filesizeformat edge cases ---

func TestFilesizeformatLargeValues(t *testing.T) {
	ctx := testCtx(t, "en")
	assert.Equal(t, "1,024.0 PB", filesizeformat(ctx, 1024*1024*1024*1024*1024*1024))
}

// --- Test with custom translation FS ---

func TestCustomTranslationOverride(t *testing.T) {
	customFS := fstest.MapFS{
		"translations/active.en.toml": &fstest.MapFile{
			Data: []byte("humanize-today = \"this very day\"\n"),
		},
	}
	b, err := i18n.NewTestBundle(translationFS, customFS)
	require.NoError(t, err)
	ctx := b.WithLocale(context.Background(), "en")

	now := time.Date(2026, 4, 1, 15, 0, 0, 0, time.UTC)
	assert.Equal(t, "this very day", naturalday(ctx, "", now, now))
}
