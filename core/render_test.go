package core

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	component := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := w.Write([]byte("<p>hello</p>"))
		return err
	})

	err := Render(c, http.StatusOK, component)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html; charset=UTF-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, "<p>hello</p>", rec.Body.String())
}

func TestRenderWithStatus(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	component := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := w.Write([]byte("<p>not found</p>"))
		return err
	})

	err := Render(c, http.StatusNotFound, component)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

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

func TestCSRFTokenContext(t *testing.T) {
	ctx := context.Background()
	ctx = WithCSRFToken(ctx, "abc123")

	assert.Equal(t, "abc123", CSRFToken(ctx))
}

func TestCSRFTokenMissing(t *testing.T) {
	ctx := context.Background()
	assert.Empty(t, CSRFToken(ctx))
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

func TestLayoutFuncWrapsContent(t *testing.T) {
	layout := LayoutFunc(func(title string, content templ.Component) templ.Component {
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			_, _ = w.Write([]byte("<html><head><title>" + title + "</title></head><body>"))
			if err := content.Render(ctx, w); err != nil {
				return err
			}
			_, err := w.Write([]byte("</body></html>"))
			return err
		})
	})

	content := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := w.Write([]byte("<p>hello</p>"))
		return err
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	wrapped := layout("Test Page", content)
	err := Render(c, http.StatusOK, wrapped)
	require.NoError(t, err)
	assert.Equal(t, "<html><head><title>Test Page</title></head><body><p>hello</p></body></html>", rec.Body.String())
}

func TestLayoutFuncNilPassthrough(t *testing.T) {
	// When no layout is set (nil in Layouts struct), content should render directly.
	layouts := Layouts{} // Both nil.

	content := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := w.Write([]byte("<p>bare</p>"))
		return err
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Simulate what an app's default renderer would do.
	var component templ.Component
	if layouts.App != nil {
		component = layouts.App("Page", content)
	} else {
		component = content
	}

	err := Render(c, http.StatusOK, component)
	require.NoError(t, err)
	assert.Equal(t, "<p>bare</p>", rec.Body.String())
}

func TestLayoutsStruct(t *testing.T) {
	appLayout := LayoutFunc(func(title string, content templ.Component) templ.Component {
		return content
	})
	adminLayout := LayoutFunc(func(title string, content templ.Component) templ.Component {
		return content
	})

	layouts := Layouts{
		App:   appLayout,
		Admin: adminLayout,
	}

	assert.NotNil(t, layouts.App)
	assert.NotNil(t, layouts.Admin)
}
