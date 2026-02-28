package templates

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/admin"
	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertion.
var _ admin.DashboardRenderer = (*defaultDashboardRenderer)(nil)

func TestLayout(t *testing.T) {
	lay := Layout()
	require.NotNil(t, lay)

	content := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, "<p>test content</p>")
		return err
	})

	component := lay("Test Page", content)
	require.NotNil(t, component)

	var buf strings.Builder
	err := component.Render(context.Background(), &buf)
	require.NoError(t, err)

	html := buf.String()
	assert.Contains(t, html, "<!doctype html>")
	assert.Contains(t, html, "<title>Test Page – admin-sidebar-title</title>")
	assert.Contains(t, html, "bootstrap.min.css")
	assert.Contains(t, html, "bootstrap-icons.min.css")
	assert.Contains(t, html, "bootstrap.bundle.min.js")
	assert.Contains(t, html, "htmx.min.js")
	assert.Contains(t, html, "<p>test content</p>")
}

func TestDefaultDashboardRendererDashboardPage(t *testing.T) {
	r := DefaultDashboardRenderer()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	err := r.DashboardPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "admin-dashboard-title")
}

func TestDefaultDashboardRendererWithLayout(t *testing.T) {
	r := DefaultDashboardRenderer()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	ctx := burrow.WithLayout(req.Context(), func(title string, content templ.Component) templ.Component {
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			_, _ = io.WriteString(w, "<layout-wrapper>")
			if err := content.Render(ctx, w); err != nil {
				return err
			}
			_, _ = io.WriteString(w, "</layout-wrapper>")
			return nil
		})
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := r.DashboardPage(rec, req)

	require.NoError(t, err)
	body := rec.Body.String()
	assert.Contains(t, body, "<layout-wrapper>")
	assert.Contains(t, body, "admin-dashboard-title")
	assert.Contains(t, body, "</layout-wrapper>")
}

func TestDefaultDashboardRendererWithNavGroups(t *testing.T) {
	r := DefaultDashboardRenderer()
	groups := []admin.NavGroup{
		{AppName: "auth", Items: []burrow.NavItem{{Label: "Users", URL: "/admin/users", Icon: "bi bi-people"}}},
	}
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	ctx := admin.WithNavGroups(req.Context(), groups)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := r.DashboardPage(rec, req)

	require.NoError(t, err)
	body := rec.Body.String()
	assert.Contains(t, body, "Users")
	assert.Contains(t, body, `href="/admin/users"`)
}
