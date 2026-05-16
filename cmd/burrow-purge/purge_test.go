package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func purge(t *testing.T, css string, classes map[string]bool) string {
	t.Helper()
	got, err := Purge([]byte(css), classes, Options{})
	require.NoError(t, err)
	return string(got)
}

func TestPurge_KeepsRuleWhenClassPresent(t *testing.T) {
	got := purge(t, `.flex { display: flex }`, map[string]bool{"flex": true})
	assert.Contains(t, got, ".flex")
	assert.Contains(t, got, "display:flex")
}

func TestPurge_DropsRuleWhenClassAbsent(t *testing.T) {
	got := purge(t, `.flex { display: flex }`, map[string]bool{"grid": true})
	assert.NotContains(t, got, "flex")
}

func TestPurge_KeepsTagSelectorRule(t *testing.T) {
	got := purge(t, `body { margin: 0 }`, map[string]bool{})
	assert.Contains(t, got, "body")
	assert.Contains(t, got, "margin:0")
}

func TestPurge_KeepsRootRule(t *testing.T) {
	got := purge(t, `:root { --color: red }`, map[string]bool{})
	assert.Contains(t, got, ":root")
	assert.Contains(t, got, "--color:red")
}

func TestPurge_KeepsUniversalSelector(t *testing.T) {
	got := purge(t, `* { box-sizing: border-box }`, map[string]bool{})
	assert.Contains(t, got, "*")
	assert.Contains(t, got, "box-sizing:border-box")
}

func TestPurge_CompoundClassRequiresAll(t *testing.T) {
	// `.a.b` only matches when both classes are present in markup.
	classes := map[string]bool{"a": true}
	got := purge(t, `.a.b { color: red }`, classes)
	assert.NotContains(t, got, "color")
}

func TestPurge_MultiSelectorSplitsAndFilters(t *testing.T) {
	// `.kept, .dropped` becomes just `.kept` after purging.
	classes := map[string]bool{"kept": true}
	got := purge(t, `.kept, .dropped { color: red }`, classes)
	assert.Contains(t, got, ".kept")
	assert.NotContains(t, got, ".dropped")
	assert.Contains(t, got, "color:red")
}

func TestPurge_KeepsFontFace(t *testing.T) {
	got := purge(t, `@font-face { font-family: x; src: url(x.woff2) }`, map[string]bool{})
	assert.Contains(t, got, "@font-face")
}

func TestPurge_KeepsKeyframes(t *testing.T) {
	css := `@keyframes spin { from { transform: rotate(0deg) } to { transform: rotate(360deg) } }`
	got := purge(t, css, map[string]bool{})
	assert.Contains(t, got, "@keyframes")
	assert.Contains(t, got, "spin")
}

func TestPurge_MediaQueryFiltersInner(t *testing.T) {
	css := `@media (min-width: 640px) { .sm\:flex { display: flex } .sm\:dropped { color: red } }`
	classes := map[string]bool{"sm:flex": true}
	got := purge(t, css, classes)
	assert.Contains(t, got, "@media")
	assert.Contains(t, got, "sm")
	// Use the escaped form to find the survivor's selector and confirm
	// the dropped sibling is gone.
	assert.True(t, strings.Contains(got, "sm\\:flex") || strings.Contains(got, ".sm"), "survivor selector should be present")
	assert.NotContains(t, got, "dropped")
}

func TestPurge_MediaQueryDroppedWhenAllInnerDropped(t *testing.T) {
	css := `@media (min-width: 640px) { .sm\:flex { display: flex } }`
	got := purge(t, css, map[string]bool{})
	assert.NotContains(t, got, "@media", "empty @media wrapper should be dropped")
}

func TestPurge_KeepsExplicitlyKeptClass(t *testing.T) {
	// --keep "manual-class" forces a class through even if not in content.
	got, err := Purge([]byte(`.manual-class { color: red }`), map[string]bool{}, Options{Keep: []string{"manual-class"}})
	require.NoError(t, err)
	assert.Contains(t, string(got), "manual-class")
}

func TestPurge_KeepsTwoIndependentRules(t *testing.T) {
	css := `.a { color: red } .b { color: blue }`
	got := purge(t, css, map[string]bool{"a": true, "b": true})
	assert.Contains(t, got, ".a")
	assert.Contains(t, got, ".b")
}

func TestPurge_DropsOneOfTwoIndependentRules(t *testing.T) {
	css := `.a { color: red } .b { color: blue }`
	got := purge(t, css, map[string]bool{"a": true})
	assert.Contains(t, got, ".a")
	assert.NotContains(t, got, ".b")
}
