package templates

import (
	"context"
	"sort"
	"strings"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/i18n"
)

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
