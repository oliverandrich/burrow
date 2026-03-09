package session_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/oliverandrich/burrow/contrib/session"
)

func ExampleInject() {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r = session.Inject(r, map[string]any{
		"username": "alice",
		"visits":   int64(42),
	})

	fmt.Println(session.GetString(r, "username"))
	fmt.Println(session.GetInt64(r, "visits"))
	// Output:
	// alice
	// 42
}

func ExampleGetValues() {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r = session.Inject(r, map[string]any{
		"theme": "dark",
	})

	values := session.GetValues(r)
	fmt.Println(values["theme"])
	// Output:
	// dark
}
