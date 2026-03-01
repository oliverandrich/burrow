package bsicons

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	err := c.Render(context.Background(), &buf)
	require.NoError(t, err)
	return buf.String()
}

func TestNamedFunction(t *testing.T) {
	c := Trash()
	require.NotNil(t, c)

	out := render(t, c)
	assert.Contains(t, out, "<svg")
	assert.Contains(t, out, "</svg>")
	assert.Contains(t, out, `fill="currentColor"`)
	assert.Contains(t, out, `width="1em"`)
	assert.Contains(t, out, `viewBox="0 0 16 16"`)
	assert.Contains(t, out, "<path")
}

func TestClassParameter(t *testing.T) {
	c := Trash("fs-1 d-block mb-2")
	out := render(t, c)
	assert.Contains(t, out, `class="fs-1 d-block mb-2"`)
}

func TestMultipleClassParameters(t *testing.T) {
	c := Trash("fs-1", "d-block", "mb-2")
	out := render(t, c)
	assert.Contains(t, out, `class="fs-1 d-block mb-2"`)
}

func TestNoClass(t *testing.T) {
	out := render(t, Trash())
	assert.NotContains(t, out, `class=`)
}

func TestDifferentIconsHaveDifferentContent(t *testing.T) {
	trash := render(t, Trash())
	house := render(t, House())
	assert.NotEqual(t, trash, house)
}
