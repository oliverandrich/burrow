package admin

import (
	"context"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
)

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

func TestRequestPathContext(t *testing.T) {
	ctx := WithRequestPath(context.Background(), "/admin/users")
	assert.Equal(t, "/admin/users", RequestPathFromContext(ctx))
}

func TestRequestPathContextEmpty(t *testing.T) {
	assert.Empty(t, RequestPathFromContext(context.Background()))
}
