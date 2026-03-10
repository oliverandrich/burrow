package i18n

import (
	"io/fs"

	"github.com/BurntSushi/toml"
	i18nlib "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// NewTestApp creates a minimal i18n App for use in tests. It initializes
// the bundle with English as default and loads translations from the given
// filesystem. This avoids the need for cli.Command-based configuration.
func NewTestApp(defaultLang string, translationFSes ...fs.FS) (*App, error) {
	tag := language.Make(defaultLang)
	a := &App{
		defaultLang: tag,
		matcher:     language.NewMatcher([]language.Tag{language.English, language.German}),
		bundle:      i18nlib.NewBundle(tag),
	}
	a.bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	for _, fsys := range translationFSes {
		if err := a.AddTranslations(fsys); err != nil {
			return nil, err
		}
	}
	return a, nil
}
