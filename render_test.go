package burrow

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderNoExecutor(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	err := Render(rec, req, http.StatusOK, "test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no template executor")
}

func TestRenderFragment(t *testing.T) {
	exec := TemplateExecutor(func(_ *http.Request, name string, _ map[string]any) (template.HTML, error) {
		return template.HTML("<p>" + name + "</p>"), nil
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	ctx := WithTemplateExecutor(req.Context(), exec)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := Render(rec, req, http.StatusOK, "greeting", nil)
	require.NoError(t, err)
	assert.Equal(t, "<p>greeting</p>", rec.Body.String())
}

func TestRenderWithLayout(t *testing.T) {
	exec := TemplateExecutor(func(_ *http.Request, name string, data map[string]any) (template.HTML, error) {
		if name == "test-layout" {
			return template.HTML("<html><body>" + string(data["Content"].(template.HTML)) + "</body></html>"), nil
		}
		return template.HTML("<p>content</p>"), nil
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	ctx := WithTemplateExecutor(req.Context(), exec)
	ctx = WithLayout(ctx, "test-layout")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := Render(rec, req, http.StatusOK, "page", nil)
	require.NoError(t, err)
	assert.Equal(t, "<html><body><p>content</p></body></html>", rec.Body.String())
}

func TestRenderHTMXSkipsLayout(t *testing.T) {
	exec := TemplateExecutor(func(_ *http.Request, _ string, _ map[string]any) (template.HTML, error) {
		return template.HTML("<p>fragment</p>"), nil
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("HX-Request", "true")
	ctx := WithTemplateExecutor(req.Context(), exec)
	ctx = WithLayout(ctx, "test-layout")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := Render(rec, req, http.StatusOK, "partial", nil)
	require.NoError(t, err)
	assert.Equal(t, "<p>fragment</p>", rec.Body.String())
}

func TestRenderBoostedRequestAppliesLayout(t *testing.T) {
	layoutCalled := false
	exec := TemplateExecutor(func(_ *http.Request, name string, data map[string]any) (template.HTML, error) {
		if name == "test-layout" {
			layoutCalled = true
			return template.HTML("<html>" + string(data["Content"].(template.HTML)) + "</html>"), nil
		}
		return template.HTML("<p>content</p>"), nil
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Boosted", "true")
	ctx := WithTemplateExecutor(req.Context(), exec)
	ctx = WithLayout(ctx, "test-layout")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := Render(rec, req, http.StatusOK, "page", nil)
	require.NoError(t, err)
	assert.True(t, layoutCalled, "layout should be called for boosted requests")
	assert.Equal(t, "<html><p>content</p></html>", rec.Body.String())
}

func TestRenderWithoutLayout(t *testing.T) {
	exec := TemplateExecutor(func(_ *http.Request, _ string, _ map[string]any) (template.HTML, error) {
		return template.HTML("<p>bare</p>"), nil
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	ctx := WithTemplateExecutor(req.Context(), exec)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := Render(rec, req, http.StatusOK, "bare", nil)
	require.NoError(t, err)
	assert.Equal(t, "<p>bare</p>", rec.Body.String())
}
