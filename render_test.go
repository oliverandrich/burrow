package burrow

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	component := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := w.Write([]byte("<p>hello</p>"))
		return err
	})

	err := Render(rec, req, http.StatusOK, component)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, "<p>hello</p>", rec.Body.String())
}

func TestRenderWithStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	component := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := w.Write([]byte("<p>not found</p>"))
		return err
	})

	err := Render(rec, req, http.StatusNotFound, component)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
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

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	wrapped := layout("Test Page", content)
	err := Render(rec, req, http.StatusOK, wrapped)
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

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// Simulate what an app's default renderer would do.
	var component templ.Component
	if layouts.App != nil {
		component = layouts.App("Page", content)
	} else {
		component = content
	}

	err := Render(rec, req, http.StatusOK, component)
	require.NoError(t, err)
	assert.Equal(t, "<p>bare</p>", rec.Body.String())
}

func TestLayoutsStruct(t *testing.T) {
	appLayout := LayoutFunc(func(_ string, content templ.Component) templ.Component {
		return content
	})
	adminLayout := LayoutFunc(func(_ string, content templ.Component) templ.Component {
		return content
	})

	layouts := Layouts{
		App:   appLayout,
		Admin: adminLayout,
	}

	assert.NotNil(t, layouts.App)
	assert.NotNil(t, layouts.Admin)
}
