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

func TestRenderNoExecutor(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	err := Render(rec, req, http.StatusOK, "test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no template executor")
}

func TestRenderFragment(t *testing.T) {
	exec := TemplateExecutor(func(_ context.Context, name string, data map[string]any) (template.HTML, error) {
		return template.HTML("<p>" + name + "</p>"), nil
	})

	ctx := WithTemplateExecutor(context.Background(), exec)
	html, err := RenderFragment(ctx, "greeting", nil)

	require.NoError(t, err)
	assert.Equal(t, template.HTML("<p>greeting</p>"), html)
}

func TestRenderFragmentWithoutExecutor(t *testing.T) {
	_, err := RenderFragment(context.Background(), "greeting", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoTemplateExecutor)
}

func TestRenderWithLayout(t *testing.T) {
	exec := TemplateExecutor(func(_ context.Context, name string, data map[string]any) (template.HTML, error) {
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
	exec := TemplateExecutor(func(_ context.Context, _ string, _ map[string]any) (template.HTML, error) {
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
	exec := TemplateExecutor(func(_ context.Context, name string, data map[string]any) (template.HTML, error) {
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
	exec := TemplateExecutor(func(_ context.Context, _ string, _ map[string]any) (template.HTML, error) {
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

func TestRenderBoostedWithCustomTarget_SkipsLayout(t *testing.T) {
	// This is the admin double-sidebar bug: hx-boost="true" with hx-target="main"
	// should NOT apply the layout, because the layout is already on the page
	// and only the <main> element is being swapped.
	layoutCalled := false
	exec := TemplateExecutor(func(_ context.Context, name string, data map[string]any) (template.HTML, error) {
		if name == "admin/layout" {
			layoutCalled = true
			return template.HTML("<html><sidebar/>" + string(data["Content"].(template.HTML)) + "</html>"), nil
		}
		return template.HTML("<p>admin content</p>"), nil
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/users", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Boosted", "true")
	req.Header.Set("HX-Target", "main")
	ctx := WithTemplateExecutor(req.Context(), exec)
	ctx = WithLayout(ctx, "admin/layout")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := Render(rec, req, http.StatusOK, "users/list", nil)
	require.NoError(t, err)
	assert.False(t, layoutCalled, "layout must NOT be applied when boosted request targets a custom element")
	assert.Equal(t, "<p>admin content</p>", rec.Body.String())
}

func TestRenderBoostedWithBodyTarget_AppliesLayout(t *testing.T) {
	// Boosted requests targeting "body" (or no target) should get the full layout.
	layoutCalled := false
	exec := TemplateExecutor(func(_ context.Context, name string, data map[string]any) (template.HTML, error) {
		if name == "app/layout" {
			layoutCalled = true
			return template.HTML("<html>" + string(data["Content"].(template.HTML)) + "</html>"), nil
		}
		return template.HTML("<p>page</p>"), nil
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Boosted", "true")
	req.Header.Set("HX-Target", "body")
	ctx := WithTemplateExecutor(req.Context(), exec)
	ctx = WithLayout(ctx, "app/layout")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := Render(rec, req, http.StatusOK, "page", nil)
	require.NoError(t, err)
	assert.True(t, layoutCalled, "layout should be applied for boosted requests targeting body")
}

func TestRenderContentBoostedWithCustomTarget_SkipsLayout(t *testing.T) {
	// Same bug but via RenderContent (used by ModelAdmin).
	exec := TemplateExecutor(func(_ context.Context, name string, data map[string]any) (template.HTML, error) {
		return template.HTML("<html><sidebar/>" + string(data["Content"].(template.HTML)) + "</html>"), nil
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/users", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Boosted", "true")
	req.Header.Set("HX-Target", "main")
	ctx := WithTemplateExecutor(req.Context(), exec)
	ctx = WithLayout(ctx, "admin/layout")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	content := template.HTML("<table>users</table>")
	err := RenderContent(rec, req, http.StatusOK, content, nil)
	require.NoError(t, err)
	assert.Equal(t, "<table>users</table>", rec.Body.String(), "should return content-only, not wrapped in layout")
}

// Benchmarks

func BenchmarkRender_Fragment(b *testing.B) {
	exec := TemplateExecutor(func(_ context.Context, _ string, _ map[string]any) (template.HTML, error) {
		return template.HTML("<p>Hello, World!</p>"), nil
	})

	ctx := WithTemplateExecutor(context.Background(), exec)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rec := httptest.NewRecorder()
		_ = Render(rec, req, http.StatusOK, "greeting", nil)
	}
}

func BenchmarkRender_WithLayout(b *testing.B) {
	exec := TemplateExecutor(func(_ context.Context, name string, data map[string]any) (template.HTML, error) {
		if name == "app/layout" {
			return template.HTML("<html><body>" + string(data["Content"].(template.HTML)) + "</body></html>"), nil
		}
		return template.HTML("<p>content</p>"), nil
	})

	ctx := WithTemplateExecutor(context.Background(), exec)
	ctx = WithLayout(ctx, "app/layout")
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	data := map[string]any{"Title": "Test Page"}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rec := httptest.NewRecorder()
		_ = Render(rec, req, http.StatusOK, "page", data)
	}
}

func BenchmarkRender_HTMXFragment(b *testing.B) {
	exec := TemplateExecutor(func(_ context.Context, _ string, _ map[string]any) (template.HTML, error) {
		return template.HTML("<p>fragment</p>"), nil
	})

	ctx := WithTemplateExecutor(context.Background(), exec)
	ctx = WithLayout(ctx, "app/layout")
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	req.Header.Set("HX-Request", "true")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rec := httptest.NewRecorder()
		_ = Render(rec, req, http.StatusOK, "partial", nil)
	}
}

func BenchmarkRenderContent_WithLayout(b *testing.B) {
	exec := TemplateExecutor(func(_ context.Context, _ string, data map[string]any) (template.HTML, error) {
		return template.HTML("<html><body>" + string(data["Content"].(template.HTML)) + "</body></html>"), nil
	})

	ctx := WithTemplateExecutor(context.Background(), exec)
	ctx = WithLayout(ctx, "app/layout")
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	content := template.HTML("<div class=\"card\"><h2>Title</h2><p>Body text here</p></div>")
	data := map[string]any{"Title": "Test"}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rec := httptest.NewRecorder()
		_ = RenderContent(rec, req, http.StatusOK, content, data)
	}
}

func BenchmarkRenderContent_NoLayout(b *testing.B) {
	ctx := WithTemplateExecutor(context.Background(), TemplateExecutor(func(_ context.Context, _ string, _ map[string]any) (template.HTML, error) {
		return "", nil
	}))
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	content := template.HTML("<div class=\"card\"><h2>Title</h2><p>Body text here</p></div>")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		rec := httptest.NewRecorder()
		_ = RenderContent(rec, req, http.StatusOK, content, nil)
	}
}
