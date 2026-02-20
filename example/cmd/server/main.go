// Command server demonstrates how to build an application
// using the burrow framework with contrib apps.
package main

import (
	"context"
	"io"
	"log"
	"os"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"codeberg.org/oliverandrich/burrow/contrib/healthcheck"
	"codeberg.org/oliverandrich/burrow/contrib/session"
	"codeberg.org/oliverandrich/burrow/example/internal/notes"
	"github.com/a-h/templ"
	"github.com/urfave/cli/v3"
)

// version is set via ldflags at build time.
var version = "dev"

// appLayout wraps page content in a minimal HTML shell.
// A real application would use a full Templ template with
// CSS framework, navigation, dark mode toggle, etc.
func appLayout(title string, content templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, _ = io.WriteString(w, "<!DOCTYPE html><html><head><title>")
		_, _ = io.WriteString(w, title)
		_, _ = io.WriteString(w, "</title></head><body>")

		// Render navigation from context.
		for _, item := range burrow.NavItems(ctx) {
			_, _ = io.WriteString(w, `<a href="`)
			_, _ = io.WriteString(w, item.URL)
			_, _ = io.WriteString(w, `">`)
			_, _ = io.WriteString(w, item.Label)
			_, _ = io.WriteString(w, `</a> `)
		}

		if err := content.Render(ctx, w); err != nil {
			return err
		}

		_, _ = io.WriteString(w, "</body></html>")
		return nil
	})
}

func main() {
	// Create the server with apps in dependency order.
	// Session must come before auth (auth depends on session).
	srv := burrow.NewServer(
		&session.App{},
		auth.New(nil), // nil renderer = no HTML pages, API-only
		&healthcheck.App{},
		notes.New(),
	)

	// Provide layouts. The App layout wraps user-facing pages,
	// the Admin layout wraps admin pages. Both are optional (nil = no wrapping).
	srv.SetLayouts(burrow.Layouts{
		App: appLayout,
	})

	cmd := &cli.Command{
		Name:    "example",
		Usage:   "Example application using the burrow framework",
		Version: version,
		Flags:   srv.Flags(nil),
		Action:  srv.Run,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
