package templates

import (
	"html/template"
	"maps"
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/admin"
	"codeberg.org/oliverandrich/burrow/contrib/i18n"
)

// Layout returns a LayoutFunc that renders page content inside the
// admin/layout template with a Bootstrap 5 sidebar and htmx.
func Layout() burrow.LayoutFunc {
	return func(w http.ResponseWriter, r *http.Request, code int, content template.HTML, data map[string]any) error {
		ctx := r.Context()
		exec := burrow.TemplateExecutorFromContext(ctx)
		if exec == nil {
			return burrow.HTML(w, code, string(content))
		}

		sidebar := admin.PrepareSidebar(ctx, admin.NavGroupsFromContext(ctx))

		layoutData := make(map[string]any, len(data)+4)
		maps.Copy(layoutData, data)
		layoutData["Content"] = content
		if _, ok := layoutData["Title"]; !ok {
			layoutData["Title"] = ""
		}
		layoutData["SidebarGroups"] = sidebar
		layoutData["ThemeSwitcherData"] = map[string]any{"Dropup": true}

		html, err := exec(r, "admin/layout", layoutData)
		if err != nil {
			return err
		}
		return burrow.HTML(w, code, string(html))
	}
}

// DefaultDashboardRenderer returns a DashboardRenderer that uses the built-in
// HTML templates for the admin dashboard page.
func DefaultDashboardRenderer() admin.DashboardRenderer {
	return &defaultDashboardRenderer{}
}

// defaultDashboardRenderer implements admin.DashboardRenderer using built-in HTML templates.
type defaultDashboardRenderer struct{}

func (d *defaultDashboardRenderer) DashboardPage(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sidebar := admin.PrepareSidebar(ctx, admin.NavGroupsFromContext(ctx))

	data := map[string]any{
		"Title":         i18n.T(ctx, "admin-sidebar-title"),
		"SidebarGroups": sidebar,
	}
	return burrow.RenderTemplate(w, r, http.StatusOK, "admin/index", data)
}
