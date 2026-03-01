package templates

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThemeScript_RendersInlineScript(t *testing.T) {
	var buf strings.Builder
	err := ThemeScript().Render(context.Background(), &buf)
	require.NoError(t, err)

	html := buf.String()
	assert.Contains(t, html, "<script>")
	assert.Contains(t, html, "localStorage")
	assert.Contains(t, html, "matchMedia")
	assert.Contains(t, html, "data-bs-theme")
	assert.NotContains(t, html, "DOMContentLoaded")
}

func TestThemeSwitcher_RendersDropdown(t *testing.T) {
	var buf strings.Builder
	err := ThemeSwitcher(false).Render(context.Background(), &buf)
	require.NoError(t, err)

	html := buf.String()

	// Renders as dropdown (not dropup).
	assert.Contains(t, html, `class="dropdown"`)

	// All three theme options are present.
	assert.Contains(t, html, `data-bs-theme-value="light"`)
	assert.Contains(t, html, `data-bs-theme-value="dark"`)
	assert.Contains(t, html, `data-bs-theme-value="auto"`)

	// SVG icons are rendered (SunFill, MoonStarsFill, CircleHalf).
	assert.Contains(t, html, "<svg")

	// JavaScript handles localStorage and matchMedia.
	assert.Contains(t, html, "localStorage")
	assert.Contains(t, html, "matchMedia")
}

func TestThemeSwitcher_RendersDropup(t *testing.T) {
	var buf strings.Builder
	err := ThemeSwitcher(true).Render(context.Background(), &buf)
	require.NoError(t, err)

	html := buf.String()
	assert.Contains(t, html, `class="dropup"`)
	assert.NotContains(t, html, `class="dropdown"`)
}
