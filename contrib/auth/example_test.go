package auth_test

import (
	"context"
	"fmt"

	"github.com/oliverandrich/burrow/contrib/auth"
)

func ExampleSafeRedirectPath() {
	// A relative path is accepted as-is.
	fmt.Println(auth.SafeRedirectPath("/profile", "/"))

	// An absolute URL with a host is rejected.
	fmt.Println(auth.SafeRedirectPath("https://evil.example.com", "/"))

	// An empty string falls back to the default.
	fmt.Println(auth.SafeRedirectPath("", "/home"))
	// Output:
	// /profile
	// /
	// /home
}

func ExampleNormalizeCode() {
	fmt.Println(auth.NormalizeCode("ABCD-EFGH-2345"))
	// Output:
	// abcdefgh2345
}

func ExampleCurrentUser() {
	user := &auth.User{Username: "alice", Role: "admin"}
	ctx := auth.WithUser(context.Background(), user)

	u := auth.CurrentUser(ctx)
	fmt.Println(u.Username, u.Role)
	// Output:
	// alice admin
}

func ExampleIsAuthenticated() {
	// No user in context — not authenticated.
	fmt.Println(auth.IsAuthenticated(context.Background()))

	// With a user in context — authenticated.
	ctx := auth.WithUser(context.Background(), &auth.User{Username: "bob"})
	fmt.Println(auth.IsAuthenticated(ctx))
	// Output:
	// false
	// true
}
