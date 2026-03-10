package i18n

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/BurntSushi/toml"
	i18nlib "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed translations
var builtinTranslationFS embed.FS

// Bundle manages i18n translation bundles and locale matching.
// It is created and managed by the Server; apps should not create it directly.
type Bundle struct {
	bundle      *i18nlib.Bundle
	matcher     language.Matcher
	defaultLang language.Tag
}

// NewBundle creates a new i18n Bundle with the given default language and
// supported languages. It loads built-in validation translations automatically.
func NewBundle(defaultLang string, supported []string) (*Bundle, error) {
	tag := language.Make(defaultLang)

	// Build tag list with default language first for proper matching.
	tags := []language.Tag{tag}
	for _, s := range supported {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		t := language.Make(s)
		if t != tag {
			tags = append(tags, t)
		}
	}

	b := &Bundle{
		defaultLang: tag,
		matcher:     language.NewMatcher(tags),
		bundle:      i18nlib.NewBundle(tag),
	}
	b.bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	if err := b.AddTranslations(builtinTranslationFS); err != nil {
		return nil, fmt.Errorf("load built-in translations: %w", err)
	}

	return b, nil
}

// AddTranslations loads translation files from an fs.FS.
// Files must be in a "translations/" directory matching "translations/*.toml".
func (b *Bundle) AddTranslations(fsys fs.FS) error {
	entries, err := fs.Glob(fsys, "translations/*.toml")
	if err != nil {
		return err
	}
	for _, path := range entries {
		if _, err := b.bundle.LoadMessageFileFS(fsys, path); err != nil {
			return err
		}
	}
	return nil
}

// WithLocale returns a new context with the given locale set.
func (b *Bundle) WithLocale(ctx context.Context, lang string) context.Context {
	tag, _, _ := b.matcher.Match(language.Make(lang))
	base, _ := tag.Base()
	locale := base.String()
	ctx = context.WithValue(ctx, ctxKeyLocale{}, locale)
	localizer := i18nlib.NewLocalizer(b.bundle, locale)
	return context.WithValue(ctx, ctxKeyLocalizer{}, localizer)
}

// LocaleMiddleware returns HTTP middleware that detects the user's locale
// from the Accept-Language header and stores it in the request context.
func (b *Bundle) LocaleMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			acceptLang := r.Header.Get("Accept-Language")
			tag, _ := language.MatchStrings(b.matcher, acceptLang)
			base, _ := tag.Base()
			locale := base.String()

			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxKeyLocale{}, locale)
			localizer := i18nlib.NewLocalizer(b.bundle, locale)
			ctx = context.WithValue(ctx, ctxKeyLocalizer{}, localizer)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestFuncMap returns request-scoped template functions for translations.
func (b *Bundle) RequestFuncMap(r *http.Request) template.FuncMap {
	ctx := r.Context()
	return template.FuncMap{
		"lang":    func() string { return Locale(ctx) },
		"t":       func(key string) string { return T(ctx, key) },
		"tData":   func(key string, data map[string]any) string { return TData(ctx, key, data) },
		"tPlural": func(key string, count int) string { return TPlural(ctx, key, count) },
	}
}
