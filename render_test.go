package burrow

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	err := Render(rec, req, http.StatusOK, template.HTML("<p>hello</p>"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, "<p>hello</p>", rec.Body.String())
}

func TestRenderWithStatus(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	err := Render(rec, req, http.StatusNotFound, template.HTML("<p>not found</p>"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRenderTemplateNoExecutor(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	err := RenderTemplate(rec, req, http.StatusOK, "test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no template executor")
}

func TestRenderTemplateFragment(t *testing.T) {
	exec := TemplateExecutor(func(_ *http.Request, name string, _ map[string]any) (template.HTML, error) {
		return template.HTML("<p>" + name + "</p>"), nil
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	ctx := WithTemplateExecutor(req.Context(), exec)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := RenderTemplate(rec, req, http.StatusOK, "greeting", nil)
	require.NoError(t, err)
	assert.Equal(t, "<p>greeting</p>", rec.Body.String())
}

func TestRenderTemplateWithLayout(t *testing.T) {
	exec := TemplateExecutor(func(_ *http.Request, _ string, _ map[string]any) (template.HTML, error) {
		return template.HTML("<p>content</p>"), nil
	})
	layout := LayoutFunc(func(w http.ResponseWriter, _ *http.Request, code int, content template.HTML, _ map[string]any) error {
		return HTML(w, code, "<html><body>"+string(content)+"</body></html>")
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	ctx := WithTemplateExecutor(req.Context(), exec)
	ctx = WithLayout(ctx, layout)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := RenderTemplate(rec, req, http.StatusOK, "page", nil)
	require.NoError(t, err)
	assert.Equal(t, "<html><body><p>content</p></body></html>", rec.Body.String())
}

func TestRenderTemplateHTMXSkipsLayout(t *testing.T) {
	exec := TemplateExecutor(func(_ *http.Request, _ string, _ map[string]any) (template.HTML, error) {
		return template.HTML("<p>fragment</p>"), nil
	})
	layoutCalled := false
	layout := LayoutFunc(func(w http.ResponseWriter, _ *http.Request, code int, content template.HTML, _ map[string]any) error {
		layoutCalled = true
		return HTML(w, code, "<html>"+string(content)+"</html>")
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("HX-Request", "true")
	ctx := WithTemplateExecutor(req.Context(), exec)
	ctx = WithLayout(ctx, layout)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := RenderTemplate(rec, req, http.StatusOK, "partial", nil)
	require.NoError(t, err)
	assert.Equal(t, "<p>fragment</p>", rec.Body.String())
	assert.False(t, layoutCalled, "layout should not be called for HTMX requests")
}

func TestRenderTemplateWithoutLayout(t *testing.T) {
	exec := TemplateExecutor(func(_ *http.Request, _ string, _ map[string]any) (template.HTML, error) {
		return template.HTML("<p>bare</p>"), nil
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	ctx := WithTemplateExecutor(req.Context(), exec)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := RenderTemplate(rec, req, http.StatusOK, "bare", nil)
	require.NoError(t, err)
	assert.Equal(t, "<p>bare</p>", rec.Body.String())
}
