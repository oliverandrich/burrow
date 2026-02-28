package templates

import (
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/admin"
	"codeberg.org/oliverandrich/burrow/contrib/i18n"
	"github.com/a-h/templ"
)

// Layout returns the batteries-included admin layout using Bootstrap 5
// and htmx. It serves as a ready-to-use admin layout that reads static file
// URLs and nav items from the request context.
func Layout() burrow.LayoutFunc {
	return layout
}

// DefaultDashboardRenderer returns a DashboardRenderer that uses the built-in
// Templ templates for the admin dashboard page.
func DefaultDashboardRenderer() admin.DashboardRenderer {
	return &defaultDashboardRenderer{}
}

// defaultDashboardRenderer implements admin.DashboardRenderer using built-in Templ templates.
type defaultDashboardRenderer struct{}

func (d *defaultDashboardRenderer) DashboardPage(w http.ResponseWriter, r *http.Request) error {
	return renderWithLayout(w, r, i18n.T(r.Context(), "admin-sidebar-title"), adminIndex())
}

// renderWithLayout wraps content in the layout from context, or renders bare content.
func renderWithLayout(w http.ResponseWriter, r *http.Request, title string, content templ.Component) error {
	lay := burrow.Layout(r.Context())
	if lay != nil {
		return burrow.Render(w, r, http.StatusOK, lay(title, content))
	}
	return burrow.Render(w, r, http.StatusOK, content)
}
