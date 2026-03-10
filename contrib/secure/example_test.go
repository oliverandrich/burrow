package secure_test

import (
	"fmt"

	"github.com/oliverandrich/burrow/contrib/secure"
)

func ExampleNew() {
	// Zero-config: sets X-Content-Type-Options, X-Frame-Options,
	// Referrer-Policy, and auto-detects HSTS from BaseURL.
	app := secure.New()
	fmt.Println(app.Name())
	// Output:
	// secure
}

func ExampleWithContentSecurityPolicy() {
	app := secure.New(
		secure.WithContentSecurityPolicy("default-src 'self'; script-src 'self'"),
		secure.WithPermissionsPolicy("camera=(), microphone=()"),
	)
	fmt.Println(app.Name())
	// Output:
	// secure
}
