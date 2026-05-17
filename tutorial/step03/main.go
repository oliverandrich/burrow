// Command step03 adds templates, a layout, and the project stylesheet.
package main

import (
	"context"
	"embed"
	"log"
	"os"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/staticfiles"
	_ "github.com/oliverandrich/den/backend/sqlite" // register sqlite:// scheme
	"github.com/urfave/cli/v3"

	"tutorial/step03/internal/app"
	"tutorial/step03/internal/polls"
)

// emptyFS is used when the application has no custom static assets at the
// framework-root level. The app shell contributes its own stylesheet via
// HasStaticFiles (served under /static/app/...).
var emptyFS embed.FS

func main() {
	staticApp, err := staticfiles.New(emptyFS)
	if err != nil {
		log.Fatal(err)
	}

	srv := burrow.NewServer(
		staticApp,
		htmx.New(),
		app.New(),
		polls.New(),
	)

	srv.SetLayout(app.Layout())

	cmd := &cli.Command{
		Name:    "polls",
		Usage:   "Polls tutorial application",
		Version: "0.3.0",
		Flags:   srv.Flags(nil),
		Action:  srv.Run,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
