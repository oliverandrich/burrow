package admin

import (
	"net/http"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/i18n"
)

// DefaultLayout returns the template name for the built-in admin layout.
func DefaultLayout() string {
	return "admin/layout"
}

// DefaultDashboardRenderer returns a DashboardRenderer that uses the built-in
// HTML templates for the admin dashboard page.
func DefaultDashboardRenderer() DashboardRenderer {
	return &defaultDashboardRenderer{}
}

// defaultDashboardRenderer implements DashboardRenderer using built-in HTML templates.
type defaultDashboardRenderer struct{}

func (d *defaultDashboardRenderer) DashboardPage(w http.ResponseWriter, r *http.Request) error {
	data := map[string]any{
		"Title": i18n.T(r.Context(), "admin-sidebar-title"),
	}
	return burrow.RenderTemplate(w, r, http.StatusOK, "admin/index", data)
}
