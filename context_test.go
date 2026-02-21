package burrow

import (
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextSetGet(t *testing.T) {
	type myKey struct{}
	ctx := context.Background()

	ctx = WithContextValue(ctx, myKey{}, "test-value")
	val, ok := ContextValue[string](ctx, myKey{})

	assert.True(t, ok)
	assert.Equal(t, "test-value", val)
}

func TestContextGetMissing(t *testing.T) {
	type myKey struct{}
	ctx := context.Background()

	val, ok := ContextValue[string](ctx, myKey{})
	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestNavItemsContext(t *testing.T) {
	ctx := context.Background()
	items := []NavItem{
		{Label: "Home", URL: "/", Position: 1},
		{Label: "About", URL: "/about", Position: 2},
	}

	ctx = WithNavItems(ctx, items)
	got := NavItems(ctx)

	require.Len(t, got, 2)
	assert.Equal(t, "Home", got[0].Label)
	assert.Equal(t, "About", got[1].Label)
}

func TestNavItemsMissing(t *testing.T) {
	ctx := context.Background()
	assert.Nil(t, NavItems(ctx))
}

func TestLayoutContext(t *testing.T) {
	layout := LayoutFunc(func(_ string, content templ.Component) templ.Component {
		return content
	})

	ctx := context.Background()
	ctx = WithLayout(ctx, layout)

	got := Layout(ctx)
	assert.NotNil(t, got)
}

func TestLayoutMissing(t *testing.T) {
	ctx := context.Background()
	assert.Nil(t, Layout(ctx))
}
