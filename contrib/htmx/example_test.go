package htmx_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/oliverandrich/burrow/contrib/htmx"
)

func ExampleRequest() {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r.Header.Set("HX-Request", "true")

	hx := htmx.Request(r)
	fmt.Println(hx.IsHTMX())
	fmt.Println(hx.IsBoosted())
	// Output:
	// true
	// false
}

func ExampleRequest_target() {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r.Header.Set("HX-Request", "true")
	r.Header.Set("HX-Target", "result-list")
	r.Header.Set("HX-Trigger-Name", "search")

	hx := htmx.Request(r)
	fmt.Println(hx.Target())
	fmt.Println(hx.TriggerName())
	// Output:
	// result-list
	// search
}

func ExampleRedirect() {
	w := httptest.NewRecorder()
	htmx.Redirect(w, "/dashboard")

	fmt.Println(w.Header().Get("HX-Redirect"))
	// Output:
	// /dashboard
}

func ExampleTrigger() {
	w := httptest.NewRecorder()
	htmx.Trigger(w, "itemAdded")

	fmt.Println(w.Header().Get("HX-Trigger"))
	// Output:
	// itemAdded
}

func ExampleReswap() {
	w := httptest.NewRecorder()
	htmx.Reswap(w, "outerHTML")

	fmt.Println(w.Header().Get("HX-Reswap"))
	// Output:
	// outerHTML
}

func ExampleRetarget() {
	w := httptest.NewRecorder()
	htmx.Retarget(w, "#error-panel")

	fmt.Println(w.Header().Get("HX-Retarget"))
	// Output:
	// #error-panel
}
