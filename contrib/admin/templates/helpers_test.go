package templates

import (
	"context"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
)

func TestGroupLabelFallback(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		want string
	}{
		{"auth", "Auth"},
		{"session", "Session"},
		{"i18n", "I18n"},
		{"a", "A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := groupLabel(ctx, tt.name)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSortNavGroups(t *testing.T) {
	ctx := context.Background()

	groups := []NavGroup{
		{AppName: "session", Items: []burrow.NavItem{{Label: "S"}}},
		{AppName: "auth", Items: []burrow.NavItem{{Label: "A"}}},
		{AppName: "i18n", Items: []burrow.NavItem{{Label: "I"}}},
	}

	sorted := sortNavGroups(ctx, groups)

	assert.Len(t, sorted, 3)
	assert.Equal(t, "auth", sorted[0].AppName)
	assert.Equal(t, "i18n", sorted[1].AppName)
	assert.Equal(t, "session", sorted[2].AppName)
}

func TestSortNavGroupsDoesNotMutateOriginal(t *testing.T) {
	ctx := context.Background()

	groups := []NavGroup{
		{AppName: "zebra", Items: []burrow.NavItem{{Label: "Z"}}},
		{AppName: "alpha", Items: []burrow.NavItem{{Label: "A"}}},
	}

	sorted := sortNavGroups(ctx, groups)

	assert.Equal(t, "zebra", groups[0].AppName, "original slice must not be mutated")
	assert.Equal(t, "alpha", sorted[0].AppName)
}

func TestSortNavGroupsEmpty(t *testing.T) {
	ctx := context.Background()
	sorted := sortNavGroups(ctx, nil)
	assert.Nil(t, sorted)
}

func TestNavGroupsContext(t *testing.T) {
	groups := []NavGroup{
		{AppName: "auth", Items: []burrow.NavItem{{Label: "Users"}}},
	}

	ctx := WithNavGroups(context.Background(), groups)
	got := NavGroupsFromContext(ctx)

	assert.Equal(t, groups, got)
}

func TestNavGroupsContextEmpty(t *testing.T) {
	got := NavGroupsFromContext(context.Background())
	assert.Nil(t, got)
}

func TestItemLabelWithKey(t *testing.T) {
	// Without i18n context, LabelKey is returned as-is by i18n.T,
	// so itemLabel falls back to Label.
	ctx := context.Background()
	item := burrow.NavItem{Label: "Users", LabelKey: "admin-nav-users"}
	assert.Equal(t, "Users", itemLabel(ctx, item))
}

func TestItemLabelWithoutKey(t *testing.T) {
	ctx := context.Background()
	item := burrow.NavItem{Label: "Dashboard"}
	assert.Equal(t, "Dashboard", itemLabel(ctx, item))
}
