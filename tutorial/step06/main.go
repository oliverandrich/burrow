// Command step06 adds an admin panel with ModelAdmin for questions.
package main

import (
	"context"
	"embed"
	"log"
	"os"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/admin"
	admintpl "codeberg.org/oliverandrich/burrow/contrib/admin/templates"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	authtpl "codeberg.org/oliverandrich/burrow/contrib/auth/templates"
	"codeberg.org/oliverandrich/burrow/contrib/bootstrap"
	"codeberg.org/oliverandrich/burrow/contrib/csrf"
	"codeberg.org/oliverandrich/burrow/contrib/healthcheck"
	"codeberg.org/oliverandrich/burrow/contrib/messages"
	"codeberg.org/oliverandrich/burrow/contrib/session"
	"codeberg.org/oliverandrich/burrow/contrib/staticfiles"
	"github.com/urfave/cli/v3"

	"tutorial/step06/internal/pages"
	"tutorial/step06/internal/polls"
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
		bootstrap.New(),
		pages.New(),
		auth.New(
			auth.WithRenderer(authtpl.DefaultRenderer()),
			auth.WithAuthLayout(authtpl.AuthLayout()),
		),
		polls.New(),
		admin.New(
			admin.WithLayout(admintpl.Layout()),
			admin.WithDashboardRenderer(admintpl.DefaultDashboardRenderer()),
		),
	)

	srv.SetLayout(pages.Layout())

	cmd := &cli.Command{
		Name:    "polls",
		Usage:   "Polls tutorial application",
		Version: "0.6.0",
		Flags:   srv.Flags(nil),
		Action:  srv.Run,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
