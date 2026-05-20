package burrow_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// forEachTranslationFile invokes fn for every active.*.toml shipped under a
// "translations/" directory anywhere in the repo. Shared by the cross-contrib
// translation guards so they stay aligned on the skip list and filters.
func forEachTranslationFile(t *testing.T, fn func(fsys fs.FS, path string)) {
	t.Helper()
	repoFS := os.DirFS(".")
	require.NoError(t, fs.WalkDir(repoFS, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
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
		fn(repoFS, path)
		return nil
	}))
}
