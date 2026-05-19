package burrow_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"
)

// TestTranslationsHaveNoValueDivergentDuplicates walks every TOML translation
// file shipped by burrow's contribs and examples and asserts that no message
// ID appears with diverging values across files for the same locale. Such a
// collision would silently overwrite at boot-time bundle-merge (last loaded
// wins) and produce a locale rendering that depends on app registration order
// — the kind of footgun that's hard to catch in per-contrib unit tests.
func TestTranslationsHaveNoValueDivergentDuplicates(t *testing.T) {
	// locale -> key -> file -> raw scalar value
	byLocale := map[string]map[string]map[string]string{}

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "site", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".toml") {
			return nil
		}
		if !strings.Contains(filepath.ToSlash(path), "/translations/") {
			return nil
		}

		locale := extractLocale(path)
		if locale == "" {
			return nil
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var decoded map[string]any
		if _, err := toml.Decode(string(raw), &decoded); err != nil {
			t.Errorf("%s: %v", path, err)
			return nil
		}

		if byLocale[locale] == nil {
			byLocale[locale] = map[string]map[string]string{}
		}
		for key, v := range decoded {
			// Only check flat scalar entries — nested tables (go-i18n plural
			// forms like [items_count]{one=…, other=…}) are out of scope for
			// the v1 check.
			s, ok := v.(string)
			if !ok {
				continue
			}
			if byLocale[locale][key] == nil {
				byLocale[locale][key] = map[string]string{}
			}
			byLocale[locale][key][path] = s
		}
		return nil
	})
	require.NoError(t, err)

	for locale, byKey := range byLocale {
		for key, byFile := range byKey {
			if len(byFile) < 2 {
				continue
			}
			values := map[string]struct{}{}
			for _, v := range byFile {
				values[v] = struct{}{}
			}
			if len(values) <= 1 {
				continue
			}
			t.Errorf("locale %q key %q has divergent values across files: %v", locale, key, byFile)
		}
	}
}

// extractLocale returns the locale tag embedded in a translation filename like
// "active.en.toml" or "active.de.toml", or "" if the file doesn't match.
func extractLocale(path string) string {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "active.") || !strings.HasSuffix(base, ".toml") {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(base, "active."), ".toml")
}
