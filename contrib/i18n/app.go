package i18n

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"codeberg.org/oliverandrich/burrow"
	"github.com/BurntSushi/toml"
	i18nlib "github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/urfave/cli/v3"
	"golang.org/x/text/language"
)

//go:embed translations
var builtinTranslationFS embed.FS

// New creates a new i18n app.
func New() *App { return &App{} }

// App implements the i18n contrib app.
type App struct {
	bundle      *i18nlib.Bundle
	matcher     language.Matcher
	defaultLang language.Tag
	registry    *burrow.Registry
}

func (a *App) Name() string { return "i18n" }
func (a *App) Register(cfg *burrow.AppConfig) error {
	a.registry = cfg.Registry
	return nil
}

func (a *App) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "i18n-default-language",
			Value:   "en",
			Usage:   "Default language",
			Sources: cli.EnvVars("I18N_DEFAULT_LANGUAGE"),
		},
		&cli.StringFlag{
			Name:    "i18n-supported-languages",
			Value:   "en,de",
			Usage:   "Comma-separated supported languages",
			Sources: cli.EnvVars("I18N_SUPPORTED_LANGUAGES"),
		},
	}
}

func (a *App) Configure(cmd *cli.Command) error {
	a.defaultLang = language.Make(cmd.String("i18n-default-language"))

	// Build tag list with default language first for proper matching.
	supported := strings.Split(cmd.String("i18n-supported-languages"), ",")
	tags := []language.Tag{a.defaultLang}
	for _, s := range supported {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		tag := language.Make(s)
		if tag != a.defaultLang {
			tags = append(tags, tag)
		}
	}

	a.matcher = language.NewMatcher(tags)
	a.bundle = i18nlib.NewBundle(a.defaultLang)
	a.bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	if err := a.AddTranslations(builtinTranslationFS); err != nil {
		return fmt.Errorf("load built-in translations: %w", err)
	}

	if a.registry != nil {
		for _, app := range a.registry.Apps() {
			if provider, ok := app.(burrow.HasTranslations); ok {
				if err := a.AddTranslations(provider.TranslationFS()); err != nil {
					return fmt.Errorf("load translations from %q: %w", app.Name(), err)
				}
			}
		}
	}

	return nil
}

// AddTranslations loads translation files from an fs.FS.
// Files must be in a "translations/" directory matching "translations/*.toml".
func (a *App) AddTranslations(fsys fs.FS) error {
	entries, err := fs.Glob(fsys, "translations/*.toml")
	if err != nil {
		return err
	}
	for _, path := range entries {
		if _, err := a.bundle.LoadMessageFileFS(fsys, path); err != nil {
			return err
		}
	}
	return nil
}

// WithLocale returns a new context with the given locale set.
func (a *App) WithLocale(ctx context.Context, lang string) context.Context {
	tag, _, _ := a.matcher.Match(language.Make(lang))
	base, _ := tag.Base()
	locale := base.String()
	ctx = context.WithValue(ctx, ctxKeyLocale{}, locale)
	localizer := i18nlib.NewLocalizer(a.bundle, locale)
	return context.WithValue(ctx, ctxKeyLocalizer{}, localizer)
}

func (a *App) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{a.localeMiddleware}
}

func (a *App) localeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptLang := r.Header.Get("Accept-Language")
		tag, _ := language.MatchStrings(a.matcher, acceptLang)
		base, _ := tag.Base()
		locale := base.String()

		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxKeyLocale{}, locale)
		localizer := i18nlib.NewLocalizer(a.bundle, locale)
		ctx = context.WithValue(ctx, ctxKeyLocalizer{}, localizer)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
