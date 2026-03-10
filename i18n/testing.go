package i18n

import (
	"io/fs"
)

// NewTestBundle creates a minimal i18n Bundle for use in tests. It initializes
// the bundle with the given default language and English+German support,
// and loads translations from the given filesystems. This avoids the need
// for Server-based configuration.
func NewTestBundle(defaultLang string, translationFSes ...fs.FS) (*Bundle, error) {
	b, err := NewBundle(defaultLang, []string{"en", "de"})
	if err != nil {
		return nil, err
	}

	for _, fsys := range translationFSes {
		if err := b.AddTranslations(fsys); err != nil {
			return nil, err
		}
	}
	return b, nil
}
