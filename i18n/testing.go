package i18n

import (
	"io/fs"
)

// NewTestBundle creates a minimal i18n Bundle for use in tests and loads
// translations from the given filesystems on top of the builtin set.
// This avoids the need for Server-based configuration.
func NewTestBundle(translationFSes ...fs.FS) (*Bundle, error) {
	b, err := NewBundle()
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
