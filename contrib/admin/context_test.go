package admin

import (
	"context"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNavItemsContext(t *testing.T) {
	ctx := context.Background()
	items := []burrow.NavItem{
		{Label: "Users", URL: "/admin/users", Position: 10},
		{Label: "Invites", URL: "/admin/invites", Position: 20},
	}

	ctx = WithNavItems(ctx, items)
	got := NavItems(ctx)

	require.Len(t, got, 2)
	assert.Equal(t, "Users", got[0].Label)
	assert.Equal(t, "Invites", got[1].Label)
}

func TestNavItemsMissing(t *testing.T) {
	ctx := context.Background()
	assert.Nil(t, NavItems(ctx))
}
