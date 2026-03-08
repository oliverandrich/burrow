package admin

import (
	"context"
	"html/template"
	"sort"
	"strings"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/i18n"
)

// SidebarGroup holds pre-computed sidebar data for template rendering.
type SidebarGroup struct {
	AppName string
	Label   string
	Items   []SidebarItem
}

// SidebarItem holds pre-computed data for a single sidebar nav link.
type SidebarItem struct {
	Label     string
	URL       string
	Icon      template.HTML
	LinkClass string
}

// PrepareSidebar pre-computes sidebar groups with translated labels and
// active-state CSS classes, ready for template rendering.
func PrepareSidebar(ctx context.Context, groups []NavGroup) []SidebarGroup {
	sorted := sortNavGroups(ctx, groups)
	if len(sorted) == 0 {
		return nil
	}

	result := make([]SidebarGroup, len(sorted))
	for i, g := range sorted {
		items := make([]SidebarItem, len(g.Items))
		for j, item := range g.Items {
			items[j] = SidebarItem{
				Label:     itemLabel(ctx, item),
				URL:       item.URL,
				Icon:      item.Icon,
				LinkClass: sidebarLinkClass(ctx, item.URL),
			}
		}
		result[i] = SidebarGroup{
			AppName: g.AppName,
			Label:   groupLabel(ctx, g.AppName),
			Items:   items,
		}
	}
	return result
}

// sortNavGroups returns a copy of groups sorted alphabetically
// by their translated display name.
func sortNavGroups(ctx context.Context, groups []NavGroup) []NavGroup {
	if len(groups) == 0 {
		return nil
	}
	sorted := make([]NavGroup, len(groups))
	copy(sorted, groups)
	sort.SliceStable(sorted, func(i, j int) bool {
		return groupLabel(ctx, sorted[i].AppName) < groupLabel(ctx, sorted[j].AppName)
	})
	return sorted
}

// groupLabel returns the translated display name for an admin app.
// It uses i18n key "admin-app-{name}" and falls back to a capitalized
// version of the app name when no translation is found.
func groupLabel(ctx context.Context, appName string) string {
	key := "admin-app-" + appName
	translated := i18n.T(ctx, key)
	if translated != key {
		return translated
	}
	return strings.ToUpper(appName[:1]) + appName[1:]
}

// itemLabel returns the translated label for a nav item.
// If LabelKey is set and translates successfully, returns the translation.
// Otherwise returns the raw Label.
func itemLabel(ctx context.Context, item burrow.NavItem) string {
	if item.LabelKey != "" {
		translated := i18n.T(ctx, item.LabelKey)
		if translated != item.LabelKey {
			return translated
		}
	}
	return item.Label
}

// sidebarLinkClass returns the CSS classes for a sidebar nav-pill link,
// adding "active" when the link matches the current request path.
func sidebarLinkClass(ctx context.Context, itemURL string) string {
	if isActivePath(ctx, itemURL) {
		return "nav-link active"
	}
	return "nav-link text-body-emphasis"
}

// isActivePath checks whether the current request path matches a nav item URL.
// It uses prefix matching so that sub-pages (e.g. /admin/users/1) highlight
// the parent nav item (/admin/users). The admin root (/admin) only matches exactly.
func isActivePath(ctx context.Context, itemURL string) bool {
	current := RequestPathFromContext(ctx)
	if current == "" || itemURL == "" {
		return false
	}
	// Exact match for the admin root to avoid highlighting it for every page.
	if itemURL == "/admin" || itemURL == "/admin/" {
		return current == "/admin" || current == "/admin/"
	}
	return strings.HasPrefix(current, itemURL)
}
