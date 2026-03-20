package admin_test

import (
	"context"
	"fmt"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/admin"
)

func ExampleWithNavGroups() {
	groups := []admin.NavGroup{
		{AppName: "users", Items: []burrow.NavItem{{Label: "All Users", URL: "/admin/users"}}},
	}
	ctx := admin.WithNavGroups(context.Background(), groups)

	result := admin.NavGroups(ctx)
	fmt.Println(result[0].AppName, result[0].Items[0].Label)
	// Output:
	// users All Users
}

func ExampleWithRequestPath() {
	ctx := admin.WithRequestPath(context.Background(), "/admin/users/42")

	fmt.Println(admin.RequestPath(ctx))
	// Output:
	// /admin/users/42
}
