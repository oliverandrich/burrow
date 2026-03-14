package burrow

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

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
	ctx := context.Background()
	ctx = WithLayout(ctx, "app/layout")

	got := Layout(ctx)
	assert.Equal(t, "app/layout", got)
}

func TestLayoutMissing(t *testing.T) {
	ctx := context.Background()
	assert.Empty(t, Layout(ctx))
}

func TestTemplateExecutorContext(t *testing.T) {
	exec := TemplateExecutor(func(_ *http.Request, name string, _ map[string]any) (template.HTML, error) {
		return template.HTML("<p>" + name + "</p>"), nil
	})

	ctx := context.Background()
	ctx = WithTemplateExecutor(ctx, exec)

	got := TemplateExec(ctx)
	require.NotNil(t, got)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	html, err := got(r, "test", nil)
	require.NoError(t, err)
	assert.Equal(t, template.HTML("<p>test</p>"), html)
}

func TestTemplateExecutorMissing(t *testing.T) {
	ctx := context.Background()
	assert.Nil(t, TemplateExec(ctx))
}

func TestWithAuthChecker(t *testing.T) {
	ctx := context.Background()
	checker := AuthChecker{
		IsAuthenticated: func() bool { return true },
		IsAdmin:         func() bool { return false },
	}

	ctx = WithAuthChecker(ctx, checker)

	assert.True(t, isAuthenticated(ctx))
	assert.False(t, isAdmin(ctx))
}

func TestAuthCheckerAdmin(t *testing.T) {
	ctx := context.Background()
	checker := AuthChecker{
		IsAuthenticated: func() bool { return true },
		IsAdmin:         func() bool { return true },
	}

	ctx = WithAuthChecker(ctx, checker)

	assert.True(t, isAuthenticated(ctx))
	assert.True(t, isAdmin(ctx))
}

func TestAuthCheckerMissing(t *testing.T) {
	ctx := context.Background()

	assert.False(t, isAuthenticated(ctx))
	assert.False(t, isAdmin(ctx))
}
