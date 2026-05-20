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
	"github.com/oliverandrich/burrow/contrib/csrf"
	"github.com/oliverandrich/burrow/contrib/healthcheck"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/oliverandrich/burrow/contrib/staticfiles"
	_ "github.com/oliverandrich/den/backend/sqlite" // register sqlite:// scheme
	"github.com/urfave/cli/v3"

	"tutorial/step07/internal/app"
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
		app.New(),
		auth.New[auth.EmptyProfile](),
		polls.New(),
		admin.New(),
	)

	srv.SetLayout(app.Layout())

	cmd := &cli.Command{
		Name:     "polls",
		Usage:    "Polls tutorial application",
		Version:  "0.7.0",
		Flags:    srv.Flags(nil),
		Action:   srv.Run,
		Commands: srv.CLICommands(),
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
