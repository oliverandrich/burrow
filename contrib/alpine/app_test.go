package alpine

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
	assert.Equal(t, "alpine", app.Name())
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

	f, err := fsys.Open("js.html")
	require.NoError(t, err, "expected js.html to exist in template FS")
	_ = f.Close()
}

func TestStaticFS(t *testing.T) {
	app := New()
	prefix, fsys := app.StaticFS()

	assert.Equal(t, "alpine", prefix)
	require.NotNil(t, fsys)

	f, err := fsys.Open("alpine.min.js")
	require.NoError(t, err, "expected alpine.min.js to exist in static FS")
	_ = f.Close()
}
