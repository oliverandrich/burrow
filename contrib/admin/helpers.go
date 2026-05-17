package admin

import (
	"context"
	"sort"
	"strings"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/i18n"
)

// DashboardGroup holds pre-computed dashboard data for template rendering.
type DashboardGroup struct {
	AppName string
	Label   string
	Items   []DashboardItem
}

// DashboardItem holds pre-computed data for a single dashboard nav link.
//
// Icon is a template name (see [burrow.NavItem]); the dashboard template
// renders it via {{ template .Icon . }}.
type DashboardItem struct {
	Label string
	URL   string
	Icon  string
}

// PrepareDashboard pre-computes dashboard groups with translated labels,
// sorted alphabetically by group name, ready for template rendering.
func PrepareDashboard(ctx context.Context, groups []NavGroup) []DashboardGroup {
	sorted := sortNavGroups(ctx, groups)
	if len(sorted) == 0 {
		return nil
	}

	result := make([]DashboardGroup, len(sorted))
	for i, g := range sorted {
		items := make([]DashboardItem, len(g.Items))
		for j, item := range g.Items {
			items[j] = DashboardItem{
				Label: itemLabel(ctx, item),
				URL:   item.URL,
				Icon:  item.Icon,
			}
		}
		result[i] = DashboardGroup{
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
