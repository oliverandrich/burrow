// Command step07 adds HTMX-powered navigation, voting, and pagination.
package main

import (
	"context"
	"embed"
	"log"
	"os"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/admin"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/burrow/contrib/bootstrap"
	"github.com/oliverandrich/burrow/contrib/csrf"
	"github.com/oliverandrich/burrow/contrib/healthcheck"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/oliverandrich/burrow/contrib/staticfiles"
	"github.com/urfave/cli/v3"

	"tutorial/step07/internal/pages"
	"tutorial/step07/internal/polls"
)

var emptyFS embed.FS

func main() {
	staticApp, err := staticfiles.New(emptyFS)
	if err != nil {
		log.Fatal(err)
	}

	srv := burrow.NewServer(
		session.New(),
		csrf.New(),
		staticApp,
		healthcheck.New(),
		messages.New(),
		htmx.New(),
		bootstrap.New(),
		pages.New(),
		auth.New(),
		polls.New(),
		admin.New(),
	)

	srv.SetLayout(pages.Layout())

	cmd := &cli.Command{
		Name:    "polls",
		Usage:   "Polls tutorial application",
		Version: "0.7.0",
		Flags:   srv.Flags(nil),
		Action:  srv.Run,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
