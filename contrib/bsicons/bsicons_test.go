package bsicons

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNamedFunction(t *testing.T) {
	out := Trash()
	assert.NotEmpty(t, out)
	assert.Contains(t, string(out), "<svg")
	assert.Contains(t, string(out), "</svg>")
	assert.Contains(t, string(out), `fill="currentColor"`)
	assert.Contains(t, string(out), `width="1em"`)
	assert.Contains(t, string(out), `viewBox="0 0 16 16"`)
	assert.Contains(t, string(out), "<path")
}

func TestClassParameter(t *testing.T) {
	out := Trash("fs-1 d-block mb-2")
	assert.Contains(t, string(out), `class="fs-1 d-block mb-2"`)
}

func TestMultipleClassParameters(t *testing.T) {
	out := Trash("fs-1", "d-block", "mb-2")
	assert.Contains(t, string(out), `class="fs-1 d-block mb-2"`)
}

func TestNoClass(t *testing.T) {
	out := Trash()
	assert.NotContains(t, string(out), `class=`)
}

func TestDifferentIconsHaveDifferentContent(t *testing.T) {
	trash := Trash()
	house := House()
	assert.NotEqual(t, trash, house)
}
