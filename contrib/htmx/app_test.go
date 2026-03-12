package htmx

import (
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ burrow.App             = (*App)(nil)
	_ burrow.HasStaticFiles  = (*App)(nil)
	_ burrow.HasTemplates    = (*App)(nil)
	_ burrow.HasDependencies = (*App)(nil)
)

func TestAppName(t *testing.T) {
	app := New()
	assert.Equal(t, "htmx", app.Name())
}

func TestAppRegister(t *testing.T) {
	app := New()
	err := app.Register(&burrow.AppConfig{})
	require.NoError(t, err)
}

func TestDependencies(t *testing.T) {
	app := New()
	assert.Equal(t, []string{"staticfiles"}, app.Dependencies())
}

func TestTemplateFSContainsExpectedFiles(t *testing.T) {
	app := New()
	fsys := app.TemplateFS()
	require.NotNil(t, fsys)

	for _, name := range []string{
		"config.html",
		"js.html",
	} {
		f, err := fsys.Open(name)
		require.NoError(t, err, "expected %s to exist in template FS", name)
		_ = f.Close()
	}
}

func TestStaticFS(t *testing.T) {
	app := New()
	prefix, fsys := app.StaticFS()

	assert.Equal(t, "htmx", prefix)
	require.NotNil(t, fsys)

	f, err := fsys.Open("htmx.min.js")
	require.NoError(t, err, "expected htmx.min.js to exist in static FS")
	_ = f.Close()
}
