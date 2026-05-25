package dev

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscover_ConventionalScaffoldLayout(t *testing.T) {
	root := t.TempDir()
	mkProject(t, root, true, true)

	cfg, err := Discover(root)
	require.NoError(t, err)

	assert.Equal(t, root, cfg.ProjectRoot)
	assert.Equal(t, "./cmd/myapp", cfg.AppPath)
	assert.Equal(t, "tailwind.css", cfg.CSSIn)
	assert.Equal(t, filepath.Join("internal", "app", "static", "app.min.css"), cfg.CSSOut)
	assert.Equal(t, []string{".go", ".html", ".css", ".toml", ".yml", ".yaml"}, cfg.WatchExts)
	assert.Equal(t, ".env", cfg.EnvFile)
	assert.Equal(t, 300*time.Millisecond, cfg.Debounce)
	assert.Equal(t, 500*time.Millisecond, cfg.KillTimeout)

	for _, want := range []string{".git", ".beans", "node_modules", "tmp", "testdata", ".tailwind"} {
		assert.Contains(t, cfg.ExcludeDirs, want, "default excludes should contain %q", want)
	}
}

func TestDiscover_MultipleCmdDirs(t *testing.T) {
	root := t.TempDir()
	mkCmd(t, root, "a")
	mkCmd(t, root, "b")

	_, err := Discover(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple")
}

func TestDiscover_NoCmdDir(t *testing.T) {
	root := t.TempDir()

	_, err := Discover(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no cmd")
}

func TestDiscover_CmdDirWithoutMainGo(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "cmd", "ghost"), 0o755))
	// no main.go inside

	_, err := Discover(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no cmd")
}

func TestDiscover_NoTailwindCSS(t *testing.T) {
	root := t.TempDir()
	mkProject(t, root, false, true)

	cfg, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, cfg.CSSIn)
	assert.Empty(t, cfg.CSSOut)
}

func TestDiscover_TailwindCSSButNoStaticDir(t *testing.T) {
	root := t.TempDir()
	mkProject(t, root, true, false)

	cfg, err := Discover(root)
	require.NoError(t, err)
	// Tailwind co-watcher silently disabled — neither in nor out is set.
	assert.Empty(t, cfg.CSSIn)
	assert.Empty(t, cfg.CSSOut)
}

func TestApplyDefaults_ExcludesCSSOutDir(t *testing.T) {
	cfg := Config{
		ProjectRoot: t.TempDir(),
		AppPath:     "./cmd/x",
		CSSIn:       "tailwind.css",
		CSSOut:      filepath.Join("internal", "app", "static", "app.min.css"),
	}
	require.NoError(t, applyDefaults(&cfg))
	assert.Contains(t, cfg.ExcludeDirs, filepath.Join("internal", "app", "static"),
		"the CSS output directory must be excluded to avoid a watcher feedback loop")
}

func TestDiscover_RefusesToRunAgainstBurrowFrameworkItself(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"),
		[]byte("module github.com/oliverandrich/burrow\n\ngo 1.26.0\n"), 0o644))
	mkCmd(t, root, "burrow")

	_, err := Discover(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "burrow framework itself")
}

func TestDiscover_AllowsRegularProjects(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"),
		[]byte("module example.com/myapp\n\ngo 1.26.0\n"), 0o644))
	mkCmd(t, root, "myapp")

	_, err := Discover(root)
	require.NoError(t, err)
}

func TestApplyDefaults_HonoursCSSOutFlagOverride(t *testing.T) {
	// Simulates a user passing --css-out web/assets/app.css after
	// auto-discovery picked a different (or no) path. applyDefaults
	// must still exclude the actual final output directory.
	cfg := Config{
		ProjectRoot: t.TempDir(),
		AppPath:     "./cmd/x",
		CSSIn:       "tailwind.css",
		CSSOut:      filepath.Join("web", "assets", "app.css"),
	}
	require.NoError(t, applyDefaults(&cfg))
	assert.Contains(t, cfg.ExcludeDirs, filepath.Join("web", "assets"))
}

// mkProject creates a temporary burrow-shaped project under root with
// the canonical "myapp" cmd entry-point. When withTailwindCSS is true
// a tailwind.css is written; when withStaticDir is true the
// conventional internal/app/static directory exists.
func mkProject(t *testing.T, root string, withTailwindCSS, withStaticDir bool) {
	t.Helper()
	mkCmd(t, root, "myapp")
	if withTailwindCSS {
		require.NoError(t, os.WriteFile(filepath.Join(root, "tailwind.css"), []byte("@import\n"), 0o644))
	}
	if withStaticDir {
		require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "app", "static"), 0o755))
	}
}

func mkCmd(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, "cmd", name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main(){}\n"), 0o644))
}
