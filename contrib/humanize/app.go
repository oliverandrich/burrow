// Package humanize provides i18n-aware template functions for human-friendly
// display of times, numbers, and file sizes.
//
// Inspired by Django's django.contrib.humanize, it registers request-scoped
// template functions that use the current locale for formatting and translation.
//
// Available template functions:
//   - naturaltime: relative time ("2 minutes ago", "in 1 hour")
//   - naturalday: "today", "yesterday", "tomorrow", or formatted date
//   - intcomma: locale-aware thousands separators (1000000 -> "1,000,000")
//   - ordinal: ordinal representation ("1st", "2nd", "3rd")
//   - apnumber: Associated Press style numbers 1-9 as words
//   - filesizeformat: human-readable file sizes ("13.2 KB", "4.1 MB")
package humanize

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"time"
)

//go:embed translations
var translationFS embed.FS

// Option configures the humanize app.
type Option func(*App)

// App implements human-friendly formatting as a burrow contrib app.
type App struct {
	dateFormat string
}

// New creates a new humanize app.
func New(opts ...Option) *App {
	a := &App{}
	for _, o := range opts {
		o(a)
	}
	return a
}

// WithDateFormat sets a custom fallback date format for [naturalday] when the
// date is not today, yesterday, or tomorrow. Uses Go's reference time layout.
// If empty (default), a locale-specific format is used.
func WithDateFormat(format string) Option {
	return func(a *App) { a.dateFormat = format }
}

func (a *App) Name() string { return "humanize" }

func (a *App) TranslationFS() fs.FS { return translationFS }

// RequestFuncMap returns request-scoped template functions for humanization.
func (a *App) RequestFuncMap(ctx context.Context) template.FuncMap {
	return template.FuncMap{
		"naturaltime":    func(t time.Time) string { return naturaltime(ctx, t, time.Now()) },
		"naturalday":     func(t time.Time) string { return naturalday(ctx, a.dateFormat, t, time.Now()) },
		"intcomma":       func(n int) string { return intcomma(ctx, n) },
		"ordinal":        func(n int) string { return ordinal(ctx, n) },
		"apnumber":       func(n int) string { return apnumber(ctx, n) },
		"filesizeformat": func(bytes int64) string { return filesizeformat(ctx, bytes) },
	}
}
