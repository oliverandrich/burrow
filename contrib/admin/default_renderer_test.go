package admin

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertion.
var _ DashboardRenderer = (*defaultDashboardRenderer)(nil)

func TestDefaultLayout(t *testing.T) {
	assert.Equal(t, "admin/layout", DefaultLayout())
}

func TestDefaultDashboardRendererDashboardPage(t *testing.T) {
	r := DefaultDashboardRenderer()

	exec := func(_ *http.Request, name string, data map[string]any) (template.HTML, error) {
		if name == "admin/layout" {
			content, _ := data["Content"].(template.HTML)
			return template.HTML("<layout>" + string(content) + "</layout>"), nil //nolint:gosec // test
		}
		return template.HTML("<dashboard:" + name + ">"), nil //nolint:gosec // test
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", nil)
	ctx := burrow.WithTemplateExecutor(req.Context(), exec)
	ctx = burrow.WithLayout(ctx, "admin/layout")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := r.DashboardPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "<layout>")
	assert.Contains(t, body, "<dashboard:admin/index>")
}

func TestDefaultDashboardRendererWithoutLayout(t *testing.T) {
	r := DefaultDashboardRenderer()

	exec := func(_ *http.Request, name string, _ map[string]any) (template.HTML, error) {
		return template.HTML("<rendered:" + name + ">"), nil //nolint:gosec // test
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", nil)
	ctx := burrow.WithTemplateExecutor(req.Context(), exec)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := r.DashboardPage(rec, req)

	require.NoError(t, err)
	assert.Contains(t, rec.Body.String(), "<rendered:admin/index>")
}

func TestDefaultDashboardRendererPreservesTitle(t *testing.T) {
	r := DefaultDashboardRenderer()

	var capturedData map[string]any
	exec := func(_ *http.Request, _ string, data map[string]any) (template.HTML, error) {
		capturedData = data
		return "ok", nil
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", nil)
	ctx := burrow.WithTemplateExecutor(req.Context(), exec)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := r.DashboardPage(rec, req)

	require.NoError(t, err)
	assert.Contains(t, capturedData, "Title")
}
