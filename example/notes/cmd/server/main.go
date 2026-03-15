// Command server demonstrates how to build an application
// using the burrow framework with contrib apps.
package main

import (
	"context"
	"embed"
	"log"
	"log/slog"
	"os"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/admin"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/burrow/contrib/bootstrap"
	"github.com/oliverandrich/burrow/contrib/csrf"
	"github.com/oliverandrich/burrow/contrib/healthcheck"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/jobs"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/oliverandrich/burrow/contrib/staticfiles"
	"github.com/oliverandrich/burrow/example/notes/internal/notes"
	"github.com/oliverandrich/burrow/example/notes/internal/pages"
	"github.com/urfave/cli/v3"
)

// version is set via ldflags at build time.
var version = "dev"

// emptyFS is an empty filesystem for staticfiles when the example has
// no user-level static assets. Contrib apps contribute their own via
// HasStaticFiles.
var emptyFS embed.FS

func main() {
	// Configure logging before starting the server. Replace with
	// tint, JSON handler, or any slog.Handler of your choice.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	// Create the staticfiles app (walks FS to compute content hashes).
	staticApp, err := staticfiles.New(emptyFS)
	if err != nil {
		log.Fatal(err)
	}

	// Create the server with apps in dependency order.
	srv := burrow.NewServer(
		session.New(),
		csrf.New(),
		staticApp,
		healthcheck.New(),
		jobs.New(),
		pages.New(),
		messages.New(),
		auth.New(
			auth.WithLogoComponent(pages.Logo()),
		),
		htmx.New(),
		bootstrap.New(),
		notes.New(),
		admin.New(),
	)

	// Use the nav layout with navbar slot (provided by bootstrap contrib).
	srv.SetLayout(bootstrap.NavLayout())

	cmd := &cli.Command{
		Name:     "example",
		Usage:    "Example application using the burrow framework",
		Version:  version,
		Flags:    srv.Flags(nil),
		Action:   srv.Run,
		Commands: srv.Registry().AllCLICommands(),
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
