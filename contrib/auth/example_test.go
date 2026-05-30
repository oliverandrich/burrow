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
	user := &auth.User[auth.EmptyProfile]{Username: "alice", Role: "admin"}
	ctx := auth.WithUser(context.Background(), user)

	u := auth.CurrentUser[auth.EmptyProfile](ctx)
	fmt.Println(u.Username, u.Role)
	// Output:
	// alice admin
}

func ExampleIsAuthenticated() {
	// No user in context — not authenticated.
	fmt.Println(auth.IsAuthenticated(context.Background()))

	// With a user in context — authenticated.
	ctx := auth.WithUser(context.Background(), &auth.User[auth.EmptyProfile]{Username: "bob"})
	fmt.Println(auth.IsAuthenticated(ctx))
	// Output:
	// false
	// true
}

func ExampleWithUsernameValidator() {
	// Reject usernames that collide with reserved handles. The returned
	// error's message is shown to the user, so keep it clean and specific.
	reserved := map[string]bool{"posts": true, "notes": true, "all": true}
	validate := func(_ context.Context, username string) error {
		if reserved[username] {
			return fmt.Errorf("username %q is reserved", username)
		}
		return nil
	}

	_ = auth.New[auth.EmptyProfile](auth.WithUsernameValidator[auth.EmptyProfile](validate))

	fmt.Println(validate(context.Background(), "posts"))
	fmt.Println(validate(context.Background(), "alice"))
	// Output:
	// username "posts" is reserved
	// <nil>
}

func ExampleWithEmailValidator() {
	// Block abusive or unwanted addresses in email mode. Uniqueness is
	// already enforced by the database — use this for policy rejections.
	validate := func(_ context.Context, email string) error {
		if email == "blocked@example.com" {
			return fmt.Errorf("email address is not allowed")
		}
		return nil
	}

	_ = auth.New[auth.EmptyProfile](auth.WithEmailValidator[auth.EmptyProfile](validate))

	fmt.Println(validate(context.Background(), "blocked@example.com"))
	fmt.Println(validate(context.Background(), "alice@example.com"))
	// Output:
	// email address is not allowed
	// <nil>
}
