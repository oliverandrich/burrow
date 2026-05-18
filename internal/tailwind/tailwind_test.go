package tailwind

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeBurrowTree builds a synthetic `<root>/contrib/` directory matching
// Burrow's contrib-app layout: each subdirectory is a contrib, some of
// which contain a `templates/` directory.
func fakeBurrowTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	contrib := filepath.Join(root, "contrib")

	for _, name := range []string{"admin", "auth", "messages"} {
		require.NoError(t, os.MkdirAll(filepath.Join(contrib, name, "templates"), 0o755))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(contrib, "notemplates"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(contrib, "README.md"), []byte("x"), 0o644))
	return root
}

// fakeProjectTree builds a synthetic project tree with both a flat
// `<root>/templates/` and an `<root>/internal/<app>/templates/` layout
// inhabited by two apps.
func fakeProjectTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(root, "templates"), 0o755))
	for _, name := range []string{"pages", "notes"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", name, "templates"), 0o755))
	}
	// An internal app without templates — must be ignored.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "cryptokey"), 0o755))
	return root
}

func TestFindTemplateSubdirs_HappyPath(t *testing.T) {
	burrowRoot := fakeBurrowTree(t)
	got := findTemplateSubdirs(filepath.Join(burrowRoot, "contrib"))
	assert.Len(t, got, 3, "three contribs in the fixture have templates")
}

func TestFindTemplateSubdirs_MissingRootReturnsNil(t *testing.T) {
	got := findTemplateSubdirs(filepath.Join(t.TempDir(), "does-not-exist"))
	assert.Empty(t, got)
}

func TestCollectTemplateDirs_BurrowOnly(t *testing.T) {
	burrowRoot := fakeBurrowTree(t)
	got := collectTemplateDirs(burrowRoot, "")
	assert.Len(t, got, 3)
	for _, dir := range got {
		assert.Contains(t, dir, "/contrib/")
		assert.True(t, strings.HasSuffix(dir, "/templates"))
	}
}

func TestCollectTemplateDirs_BurrowAndProject(t *testing.T) {
	burrowRoot := fakeBurrowTree(t)
	projectRoot := fakeProjectTree(t)
	got := collectTemplateDirs(burrowRoot, projectRoot)

	// 3 contribs + 1 flat templates + 2 internal apps = 6
	assert.Len(t, got, 6)

	// All entries are absolute and sorted.
	for _, dir := range got {
		assert.True(t, filepath.IsAbs(dir))
	}
	sorted := append([]string(nil), got...)
	require.Equal(t, sorted, got, "collectTemplateDirs must return a sorted slice")
}

func TestCollectTemplateDirs_ProjectWithoutInternal(t *testing.T) {
	burrowRoot := fakeBurrowTree(t)
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "templates"), 0o755))

	got := collectTemplateDirs(burrowRoot, projectRoot)
	// 3 contribs + 1 flat templates = 4
	assert.Len(t, got, 4)
}

func TestCollectTemplateDirs_ProjectWithoutTemplates(t *testing.T) {
	burrowRoot := fakeBurrowTree(t)
	projectRoot := t.TempDir()
	// Empty project — no templates/, no internal/

	got := collectTemplateDirs(burrowRoot, projectRoot)
	// Just the 3 burrow contribs.
	assert.Len(t, got, 3)
}

func TestWriteSources_EmitsAtSourcePerDir(t *testing.T) {
	dirs := []string{"/abs/contrib/admin/templates", "/abs/internal/notes/templates"}
	out := filepath.Join(t.TempDir(), ".tailwind", "sources.css")
	require.NoError(t, writeSources(out, dirs))

	data, err := os.ReadFile(out)
	require.NoError(t, err)
	s := string(data)
	assert.Contains(t, s, `@source "/abs/contrib/admin/templates";`)
	assert.Contains(t, s, `@source "/abs/internal/notes/templates";`)
}

func TestWriteSources_WritesUnderNewParentDir(t *testing.T) {
	out := filepath.Join(t.TempDir(), "deep", "nested", "sources.css")
	require.NoError(t, writeSources(out, []string{"/abs/contrib/admin/templates"}))
	_, err := os.Stat(out)
	require.NoError(t, err)
}

func TestWriteSources_OutputStartsWithHeaderComment(t *testing.T) {
	out := filepath.Join(t.TempDir(), "sources.css")
	require.NoError(t, writeSources(out, nil))

	data, err := os.ReadFile(out)
	require.NoError(t, err)
	first := strings.SplitN(string(data), "\n", 2)[0]
	assert.Contains(t, first, "Auto-generated")
}

func TestSourcesOutPath(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no -i falls back to cwd", []string{"-o", "out.css", "--minify"}, ".tailwind/sources.css"},
		{"-i with separate value", []string{"-i", "example/notes/tailwind.css", "-o", "x"}, "example/notes/.tailwind/sources.css"},
		{"--input long form", []string{"--input", "pkg/tailwind.css"}, "pkg/.tailwind/sources.css"},
		{"-i= attached form", []string{"-i=example/hello/tailwind.css"}, "example/hello/.tailwind/sources.css"},
		{"--input= attached form", []string{"--input=app/tailwind.css"}, "app/.tailwind/sources.css"},
		{"bare filename", []string{"-i", "tailwind.css"}, ".tailwind/sources.css"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, sourcesOutPath(tc.args))
		})
	}
}
