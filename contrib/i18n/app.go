package i18n

import (
	"context"
	"io/fs"
	"strings"

	"codeberg.org/oliverandrich/burrow"
	"github.com/BurntSushi/toml"
	"github.com/labstack/echo/v5"
	i18nlib "github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/urfave/cli/v3"
	"golang.org/x/text/language"
)

// App implements the i18n contrib app.
type App struct {
	bundle      *i18nlib.Bundle
	matcher     language.Matcher
	defaultLang language.Tag
}

func (a *App) Name() string                       { return "i18n" }
func (a *App) Register(_ *burrow.AppConfig) error { return nil }

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

func (a *App) Middleware() []echo.MiddlewareFunc {
	return []echo.MiddlewareFunc{a.localeMiddleware}
}

func (a *App) localeMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		acceptLang := c.Request().Header.Get("Accept-Language")
		tag, _ := language.MatchStrings(a.matcher, acceptLang)
		base, _ := tag.Base()
		locale := base.String()

		ctx := c.Request().Context()
		ctx = context.WithValue(ctx, ctxKeyLocale{}, locale)
		localizer := i18nlib.NewLocalizer(a.bundle, locale)
		ctx = context.WithValue(ctx, ctxKeyLocalizer{}, localizer)
		c.SetRequest(c.Request().WithContext(ctx))

		return next(c)
	}
}
