package csrf_test

import (
	"context"
	"fmt"

	"github.com/oliverandrich/burrow/contrib/csrf"
)

func ExampleWithToken() {
	ctx := csrf.WithToken(context.Background(), "abc123")

	fmt.Println(csrf.Token(ctx))
	// Output:
	// abc123
}

func ExampleToken() {
	// Without a token in the context, Token returns an empty string.
	fmt.Println(csrf.Token(context.Background()) == "")
	// Output:
	// true
}
