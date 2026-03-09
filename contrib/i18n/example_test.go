package i18n_test

import (
	"context"
	"fmt"

	"github.com/oliverandrich/burrow/contrib/i18n"
)

func ExampleLocale() {
	// Without a locale in context, Locale defaults to "en".
	fmt.Println(i18n.Locale(context.Background()))
	// Output:
	// en
}

func ExampleT_fallback() {
	// Without a localizer, T returns the message key as-is.
	fmt.Println(i18n.T(context.Background(), "greeting.hello"))
	// Output:
	// greeting.hello
}
