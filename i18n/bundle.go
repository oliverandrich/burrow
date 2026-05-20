package i18n

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/BurntSushi/toml"
	i18nlib "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed translations
var builtinTranslationFS embed.FS

// Bundle manages i18n translation bundles and locale matching.
// It is created and managed by the Server; apps should not create it directly.
type Bundle struct {
	bundle  *i18nlib.Bundle
	matcher language.Matcher
}

// NewBundle creates a new i18n Bundle. The set of supported languages is
// derived from the locales actually shipped by registered translation
// filesystems — every [Bundle.AddTranslations] call extends the matcher
// with the locales it loads. The bundle's source-language tag is
// hardcoded to English; with the Label-as-key convention, the literal
// rendered on a translation miss IS the source-language string, so the
// bundle tag is only a plural-rule fallback.
func NewBundle() (*Bundle, error) {
	b := &Bundle{
		bundle:  i18nlib.NewBundle(language.English),
		matcher: language.NewMatcher([]language.Tag{language.English}),
	}
	b.bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	if err := b.AddTranslations(builtinTranslationFS); err != nil {
		return nil, fmt.Errorf("load built-in translations: %w", err)
	}

	return b, nil
}

// AddTranslations loads translation files from an fs.FS. Files must be
// in a "translations/" directory matching "translations/*.toml". Each
// loaded file's locale becomes reachable via [Bundle.WithLocale] and
// [Bundle.LocaleMiddleware].
func (b *Bundle) AddTranslations(fsys fs.FS) error {
	entries, err := fs.Glob(fsys, "translations/*.toml")
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	for _, path := range entries {
		if _, err := b.bundle.LoadMessageFileFS(fsys, path); err != nil {
			return err
		}
	}
	b.matcher = language.NewMatcher(b.bundle.LanguageTags())
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

// RequestFuncMap returns context-scoped template functions for translations.
func (b *Bundle) RequestFuncMap(ctx context.Context) template.FuncMap {
	return template.FuncMap{
		"lang":    func() string { return Locale(ctx) },
		"t":       func(key string) string { return T(ctx, key) },
		"tData":   func(key string, data map[string]any) string { return TData(ctx, key, data) },
		"tPlural": func(key string, count int) string { return TPlural(ctx, key, count) },
	}
}
