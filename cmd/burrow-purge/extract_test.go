package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractClasses_DoubleQuoted(t *testing.T) {
	got := ExtractClasses([]byte(`<div class="flex items-center">x</div>`))
	assert.True(t, got["flex"])
	assert.True(t, got["items-center"])
}

func TestExtractClasses_SingleQuoted(t *testing.T) {
	got := ExtractClasses([]byte(`<div class='grid grid-cols-3'>x</div>`))
	assert.True(t, got["grid"])
	assert.True(t, got["grid-cols-3"])
}

func TestExtractClasses_MultipleAttributesAcrossDocument(t *testing.T) {
	got := ExtractClasses([]byte(`
<div class="a b">
    <span class="c"></span>
</div>
<p class="d e f"></p>
`))
	for _, want := range []string{"a", "b", "c", "d", "e", "f"} {
		assert.True(t, got[want], "expected class %q to be extracted", want)
	}
}

func TestExtractClasses_TailwindVariants(t *testing.T) {
	got := ExtractClasses([]byte(`<div class="hover:bg-red-500 lg:grid-cols-4 dark:text-white">x</div>`))
	assert.True(t, got["hover:bg-red-500"])
	assert.True(t, got["lg:grid-cols-4"])
	assert.True(t, got["dark:text-white"])
}

func TestExtractClasses_FractionUtilities(t *testing.T) {
	got := ExtractClasses([]byte(`<div class="w-1/2 h-2/3">x</div>`))
	assert.True(t, got["w-1/2"])
	assert.True(t, got["h-2/3"])
}

func TestExtractClasses_TemplateConditional(t *testing.T) {
	// pongo2 / django-style conditionals inside class= still scan as a string,
	// so the tokens inside survive extraction unchanged.
	got := ExtractClasses([]byte(`<a class="btn {% if active %}border-blue-500{% endif %}">x</a>`))
	assert.True(t, got["btn"])
	assert.True(t, got["border-blue-500"])
}

func TestExtractClasses_ExtraWhitespace(t *testing.T) {
	got := ExtractClasses([]byte(`<div class="  flex   gap-4  ">x</div>`))
	assert.True(t, got["flex"])
	assert.True(t, got["gap-4"])
	assert.False(t, got[""], "empty token must not be extracted")
}

func TestExtractClasses_NoClassAttributes(t *testing.T) {
	got := ExtractClasses([]byte(`<div id="main"><p>hello</p></div>`))
	assert.Empty(t, got)
}

func TestExtractClasses_IgnoresEmptyClassAttribute(t *testing.T) {
	got := ExtractClasses([]byte(`<div class="">x</div>`))
	assert.Empty(t, got)
}
