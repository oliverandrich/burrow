package admin

import (
	"context"
	"html/template"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestIsActivePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		itemURL string
		want    bool
	}{
		{"exact match", "/admin/users", "/admin/users", true},
		{"sub-path match", "/admin/users/1", "/admin/users", true},
		{"no match", "/admin/invites", "/admin/users", false},
		{"admin root exact", "/admin", "/admin", true},
		{"admin root no false positive", "/admin/users", "/admin", false},
		{"empty path", "", "/admin/users", false},
		{"empty item URL", "/admin/users", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithRequestPath(context.Background(), tt.path)
			assert.Equal(t, tt.want, isActivePath(ctx, tt.itemURL))
		})
	}
}

func TestSidebarLinkClass(t *testing.T) {
	ctx := WithRequestPath(context.Background(), "/admin/users")

	active := sidebarLinkClass(ctx, "/admin/users")
	assert.Equal(t, "nav-link active", active)

	inactive := sidebarLinkClass(ctx, "/admin/invites")
	assert.Equal(t, "nav-link text-body-emphasis", inactive)
}

func TestPrepareSidebar(t *testing.T) {
	ctx := WithRequestPath(context.Background(), "/admin/users")

	groups := []NavGroup{
		{AppName: "auth", Items: []burrow.NavItem{
			{Label: "Users", URL: "/admin/users", Icon: template.HTML("<svg>users</svg>")},
			{Label: "Invites", URL: "/admin/invites", Icon: template.HTML("<svg>invites</svg>")},
		}},
	}

	sidebar := PrepareSidebar(ctx, groups)

	require.Len(t, sidebar, 1)
	assert.Equal(t, "Auth", sidebar[0].Label)
	assert.Equal(t, "auth", sidebar[0].AppName)

	require.Len(t, sidebar[0].Items, 2)
	assert.Equal(t, "Users", sidebar[0].Items[0].Label)
	assert.Equal(t, "nav-link active", sidebar[0].Items[0].LinkClass)
	assert.Equal(t, template.HTML("<svg>users</svg>"), sidebar[0].Items[0].Icon)

	assert.Equal(t, "Invites", sidebar[0].Items[1].Label)
	assert.Equal(t, "nav-link text-body-emphasis", sidebar[0].Items[1].LinkClass)
}

func TestPrepareSidebarEmpty(t *testing.T) {
	ctx := context.Background()
	sidebar := PrepareSidebar(ctx, nil)
	assert.Nil(t, sidebar)
}
