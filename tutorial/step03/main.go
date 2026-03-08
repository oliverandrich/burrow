// Command step03 adds templates, layouts, and Bootstrap styling.
package main

import (
	"context"
	"embed"
	"log"
	"os"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/bootstrap"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/staticfiles"
	"github.com/urfave/cli/v3"

	"tutorial/step03/internal/pages"
	"tutorial/step03/internal/polls"
)

// emptyFS is used when the application has no custom static assets.
// Contrib apps like bootstrap contribute their own assets automatically.
var emptyFS embed.FS

func main() {
	staticApp, err := staticfiles.New(emptyFS)
	if err != nil {
		log.Fatal(err)
	}

	srv := burrow.NewServer(
		staticApp,
		htmx.New(),
		bootstrap.New(),
		pages.New(),
		polls.New(),
	)

	srv.SetLayout(pages.Layout())

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
