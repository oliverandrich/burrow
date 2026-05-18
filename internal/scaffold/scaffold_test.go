package scaffold_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/oliverandrich/burrow/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_CopiesPlainFile(t *testing.T) {
	src := fstest.MapFS{
		"hello.txt": {Data: []byte("hello world")},
	}
	dest := t.TempDir()

	err := scaffold.Render(src, dest, nil, nil)
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(dest, "hello.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(got))
}

func TestRender_SubstitutesPlaceholdersInContent(t *testing.T) {
	src := fstest.MapFS{
		"README.md": {Data: []byte("# __ProjectName__\n\n__ProjectDescription__")},
	}
	dest := t.TempDir()
	vars := scaffold.Vars{
		"__ProjectName__":        "demo",
		"__ProjectDescription__": "a demo project",
	}

	err := scaffold.Render(src, dest, vars, nil)
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(dest, "README.md"))
	require.NoError(t, err)
	assert.Equal(t, "# demo\n\na demo project", string(got))
}

func TestRender_SubstitutesPlaceholdersInPath(t *testing.T) {
	src := fstest.MapFS{
		"cmd/__ProjectName__/main.go": {Data: []byte("package main")},
	}
	dest := t.TempDir()
	vars := scaffold.Vars{"__ProjectName__": "demo"}

	err := scaffold.Render(src, dest, vars, nil)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dest, "cmd", "demo", "main.go"))
	assert.NoFileExists(t, filepath.Join(dest, "cmd", "__ProjectName__", "main.go"))
}

func TestRender_RewritesModulePathInGoFile(t *testing.T) {
	src := fstest.MapFS{
		"main.go": {Data: []byte(`package main

import (
	"github.com/oliverandrich/go-burrow-template/internal/app"
)

var _ = app.New
`)},
	}
	dest := t.TempDir()
	rewrite := &scaffold.ModuleRewrite{
		From: "github.com/oliverandrich/go-burrow-template",
		To:   "github.com/me/demo",
	}

	err := scaffold.Render(src, dest, nil, rewrite)
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(dest, "main.go"))
	require.NoError(t, err)
	assert.Contains(t, string(got), `"github.com/me/demo/internal/app"`)
	assert.NotContains(t, string(got), "go-burrow-template")
}

func TestRender_RewritesModulePathInGoMod(t *testing.T) {
	src := fstest.MapFS{
		"go.mod": {Data: []byte("module github.com/oliverandrich/go-burrow-template\n\ngo 1.26\n")},
	}
	dest := t.TempDir()
	rewrite := &scaffold.ModuleRewrite{
		From: "github.com/oliverandrich/go-burrow-template",
		To:   "github.com/me/demo",
	}

	err := scaffold.Render(src, dest, nil, rewrite)
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(dest, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "module github.com/me/demo\n")
}

func TestRender_DoesNotRewriteModulePathInHTML(t *testing.T) {
	src := fstest.MapFS{
		"templates/page.html": {Data: []byte(`<a href="https://github.com/oliverandrich/go-burrow-template">repo</a>`)},
	}
	dest := t.TempDir()
	rewrite := &scaffold.ModuleRewrite{
		From: "github.com/oliverandrich/go-burrow-template",
		To:   "github.com/me/demo",
	}

	err := scaffold.Render(src, dest, nil, rewrite)
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(dest, "templates", "page.html"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "go-burrow-template", "module rewrite must only apply to .go and go.mod")
}

func TestRender_AppliesContentSubstitutionAndModuleRewriteTogether(t *testing.T) {
	src := fstest.MapFS{
		"cmd/__ProjectName__/main.go": {Data: []byte(`package main

import "github.com/oliverandrich/go-burrow-template/internal/app"

const name = "__ProjectName__"

var _ = app.New
`)},
	}
	dest := t.TempDir()
	vars := scaffold.Vars{"__ProjectName__": "demo"}
	rewrite := &scaffold.ModuleRewrite{
		From: "github.com/oliverandrich/go-burrow-template",
		To:   "github.com/me/demo",
	}

	err := scaffold.Render(src, dest, vars, rewrite)
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(dest, "cmd", "demo", "main.go"))
	require.NoError(t, err)
	assert.Contains(t, string(got), `"github.com/me/demo/internal/app"`)
	assert.Contains(t, string(got), `const name = "demo"`)
}

func TestRender_RefusesNonEmptyDestination(t *testing.T) {
	src := fstest.MapFS{
		"hello.txt": {Data: []byte("hi")},
	}
	dest := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dest, "preexisting"), []byte("x"), 0o644))

	err := scaffold.Render(src, dest, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not empty")
}

func TestRender_StripsTmplSuffixFromOutputPath(t *testing.T) {
	src := fstest.MapFS{
		"go.mod.tmpl": {Data: []byte("module github.com/oliverandrich/go-burrow-template\n")},
	}
	dest := t.TempDir()
	rewrite := &scaffold.ModuleRewrite{
		From: "github.com/oliverandrich/go-burrow-template",
		To:   "github.com/me/demo",
	}

	err := scaffold.Render(src, dest, nil, rewrite)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dest, "go.mod"))
	assert.NoFileExists(t, filepath.Join(dest, "go.mod.tmpl"))

	got, err := os.ReadFile(filepath.Join(dest, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "module github.com/me/demo", "module rewrite must apply to go.mod.tmpl source files")
}

func TestRender_CreatesIntermediateDirectories(t *testing.T) {
	src := fstest.MapFS{
		"a/b/c/deep.txt": {Data: []byte("ok")},
	}
	dest := t.TempDir()

	err := scaffold.Render(src, dest, nil, nil)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(dest, "a", "b", "c", "deep.txt"))
}
