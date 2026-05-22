package translationscheck_test

import (
	"io/fs"
	"testing"

	"github.com/BurntSushi/toml"
	i18nlib "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// TestTranslationsLoadInGoI18n loads every contrib translation file through a
// real go-i18n bundle to catch reserved top-level keys ("id", "hash",
// "description", "leftdelim", "rightdelim", plural keys) that make go-i18n
// reject the file at boot.
func TestTranslationsLoadInGoI18n(t *testing.T) {
	forEachTranslationFile(t, func(fsys fs.FS, path string) {
		bundle := i18nlib.NewBundle(language.English)
		bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
		if _, err := bundle.LoadMessageFileFS(fsys, path); err != nil {
			t.Errorf("%s: go-i18n rejected the bundle: %v", path, err)
		}
	})
}
