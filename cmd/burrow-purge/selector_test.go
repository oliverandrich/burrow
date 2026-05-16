package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelectorClasses_SingleClass(t *testing.T) {
	assert.Equal(t, []string{"flex"}, selectorClasses(".flex"))
}

func TestSelectorClasses_Compound(t *testing.T) {
	assert.ElementsMatch(t, []string{"foo", "bar"}, selectorClasses(".foo.bar"))
}

func TestSelectorClasses_Descendant(t *testing.T) {
	assert.ElementsMatch(t, []string{"outer", "inner"}, selectorClasses(".outer .inner"))
}

func TestSelectorClasses_WithPseudo(t *testing.T) {
	assert.Equal(t, []string{"flex"}, selectorClasses(".flex:hover"))
}

func TestSelectorClasses_WithTagPrefix(t *testing.T) {
	assert.Equal(t, []string{"big"}, selectorClasses("h1.big"))
}

func TestSelectorClasses_EscapedColon(t *testing.T) {
	// Tailwind hover variant: `.hover\:bg-red-500` in CSS encodes the
	// `hover:bg-red-500` class. The colon is escaped.
	assert.Equal(t, []string{"hover:bg-red-500"}, selectorClasses(`.hover\:bg-red-500`))
}

func TestSelectorClasses_EscapedSlash(t *testing.T) {
	// Tailwind fraction: `.w-1\/2` encodes `w-1/2`.
	assert.Equal(t, []string{"w-1/2"}, selectorClasses(`.w-1\/2`))
}

func TestSelectorClasses_TagOnly(t *testing.T) {
	assert.Empty(t, selectorClasses("body"))
	assert.Empty(t, selectorClasses("html"))
	assert.Empty(t, selectorClasses("*"))
	assert.Empty(t, selectorClasses("a"))
}

func TestSelectorClasses_PseudoOnly(t *testing.T) {
	assert.Empty(t, selectorClasses(":root"))
	assert.Empty(t, selectorClasses("::before"))
}

func TestSelectorClasses_AttributeSelector(t *testing.T) {
	// `[data-theme="dark"]` is class-free; the actual class lives in
	// `.dark\:bg-red-500`. Only the class name comes out.
	assert.Equal(t, []string{"dark:bg-red-500"}, selectorClasses(`[data-theme="dark"] .dark\:bg-red-500`))
}

func TestKeepRule_NoClasses(t *testing.T) {
	// Tag/pseudo-only selectors are always kept regardless of content set.
	assert.True(t, keepRule("body", map[string]bool{}))
	assert.True(t, keepRule(":root", map[string]bool{}))
	assert.True(t, keepRule("*", map[string]bool{}))
}

func TestKeepRule_AllClassesPresent(t *testing.T) {
	classes := map[string]bool{"flex": true, "items-center": true}
	assert.True(t, keepRule(".flex.items-center", classes))
}

func TestKeepRule_MissingClass(t *testing.T) {
	classes := map[string]bool{"flex": true}
	assert.False(t, keepRule(".flex.items-center", classes))
}

func TestKeepRule_DescendantAllPresent(t *testing.T) {
	classes := map[string]bool{"outer": true, "inner": true}
	assert.True(t, keepRule(".outer .inner", classes))
}

func TestSplitSelectorList(t *testing.T) {
	// `.a, .b, .c:hover` → 3 selectors
	got := splitSelectorList(".a, .b, .c:hover")
	assert.Equal(t, []string{".a", ".b", ".c:hover"}, got)
}

func TestSplitSelectorList_HandlesCommasInBrackets(t *testing.T) {
	// `:is(.a, .b) .c` is one selector with a function call containing a comma.
	got := splitSelectorList(":is(.a, .b) .c")
	assert.Equal(t, []string{":is(.a, .b) .c"}, got)
}
