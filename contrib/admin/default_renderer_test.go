package admin

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertion.
var _ DashboardRenderer = (*defaultDashboardRenderer)(nil)

// stubLayoutExecutor returns a TemplateExecutor that echoes the template name
// wrapped in a tag, ignoring the actual template content.
func stubLayoutExecutor(_ *http.Request, name string, data map[string]any) (template.HTML, error) {
	content, _ := data["Content"].(template.HTML)
	return template.HTML("<rendered:" + name + ">" + string(content) + "</rendered:" + name + ">"), nil //nolint:gosec // test
}

func TestDefaultLayout(t *testing.T) {
	lay := DefaultLayout()
	require.NotNil(t, lay)

	groups := []NavGroup{
		{AppName: "auth", Items: []burrow.NavItem{{Label: "Users", URL: "/admin/users"}}},
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", nil)
	ctx := burrow.WithTemplateExecutor(req.Context(), stubLayoutExecutor)
	ctx = WithNavGroups(ctx, groups)
	ctx = WithRequestPath(ctx, "/admin/users")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	err := lay(rec, req, http.StatusOK, "<p>content</p>", nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "<rendered:admin/layout>")
	assert.Contains(t, body, "<p>content</p>")
}

func TestDefaultLayoutWithoutExecutor(t *testing.T) {
	lay := DefaultLayout()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	err := lay(rec, req, http.StatusOK, "<p>fallback</p>", nil)

	require.NoError(t, err)
	body := rec.Body.String()
	assert.Contains(t, body, "<p>fallback</p>")
}

func TestDefaultLayoutPreservesTitleFromData(t *testing.T) {
	lay := DefaultLayout()

	var capturedData map[string]any
	exec := func(_ *http.Request, _ string, data map[string]any) (template.HTML, error) {
		capturedData = data
		return "ok", nil
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", nil)
	ctx := burrow.WithTemplateExecutor(req.Context(), exec)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	err := lay(rec, req, http.StatusOK, "", map[string]any{"Title": "Custom Title"})

	require.NoError(t, err)
	assert.Equal(t, "Custom Title", capturedData["Title"])
}

func TestDefaultDashboardRendererDashboardPage(t *testing.T) {
	r := DefaultDashboardRenderer()

	exec := func(_ *http.Request, name string, _ map[string]any) (template.HTML, error) {
		return template.HTML("<dashboard:" + name + ">"), nil //nolint:gosec // test
	}

	lay := func(w http.ResponseWriter, _ *http.Request, code int, content template.HTML, _ map[string]any) error {
		return burrow.HTML(w, code, "<layout>"+string(content)+"</layout>")
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", nil)
	ctx := burrow.WithTemplateExecutor(req.Context(), exec)
	ctx = burrow.WithLayout(ctx, lay)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := r.DashboardPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "<layout>")
	assert.Contains(t, body, "<dashboard:admin/index>")
}

func TestDefaultDashboardRendererWithNavGroups(t *testing.T) {
	r := DefaultDashboardRenderer()

	var capturedData map[string]any
	exec := func(_ *http.Request, _ string, data map[string]any) (template.HTML, error) {
		capturedData = data
		return "ok", nil
	}

	groups := []NavGroup{
		{AppName: "auth", Items: []burrow.NavItem{{Label: "Users", URL: "/admin/users"}}},
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", nil)
	ctx := burrow.WithTemplateExecutor(req.Context(), exec)
	ctx = WithNavGroups(ctx, groups)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := r.DashboardPage(rec, req)

	require.NoError(t, err)
	sidebar, ok := capturedData["SidebarGroups"].([]SidebarGroup)
	require.True(t, ok)
	require.Len(t, sidebar, 1)
	assert.Equal(t, "Auth", sidebar[0].Label)
}
