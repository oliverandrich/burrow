package litewind

import (
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppName(t *testing.T) {
	assert.Equal(t, "litewind", New().Name())
}

func TestDependencies(t *testing.T) {
	assert.Equal(t, []string{"staticfiles", "htmx"}, New().Dependencies())
}

func TestConfigureRegistersThemeIcons(t *testing.T) {
	cfg := &burrow.AppConfig{}
	require.NoError(t, New().Configure(cfg, nil))

	icons := cfg.IconFuncs()
	assert.Contains(t, icons, "iconSunFill")
	assert.Contains(t, icons, "iconMoonStarsFill")
	assert.Contains(t, icons, "iconCircleHalf")
}

func TestStaticFS(t *testing.T) {
	prefix, fsys := New().StaticFS()
	assert.Equal(t, "litewind", prefix)
	require.NotNil(t, fsys)

	f, err := fsys.Open("litewind.min.css")
	require.NoError(t, err, "litewind.min.css must be present in the embedded static FS")
	_ = f.Close()
}

func TestTemplateFS(t *testing.T) {
	fsys := New().TemplateFS()
	require.NotNil(t, fsys)

	for _, name := range []string{"css.html", "theme_script.html", "theme_switcher.html"} {
		f, err := fsys.Open(name)
		require.NoError(t, err, "expected %s in template FS", name)
		_ = f.Close()
	}
}
