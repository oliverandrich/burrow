// Command server demonstrates how to build an application
// using the burrow framework with contrib apps.
package main

import (
	"context"
	"embed"
	"log"
	"log/slog"
	"os"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/admin"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"codeberg.org/oliverandrich/burrow/contrib/bootstrap"
	"codeberg.org/oliverandrich/burrow/contrib/csrf"
	"codeberg.org/oliverandrich/burrow/contrib/healthcheck"
	"codeberg.org/oliverandrich/burrow/contrib/session"
	"codeberg.org/oliverandrich/burrow/contrib/staticfiles"
	"codeberg.org/oliverandrich/burrow/example/internal/notes"
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

	// Create the auth app with default renderers (batteries-included templates).
	authApp := auth.New(auth.DefaultRenderer())

	// Create the server with apps in dependency order.
	// Session must come before auth (auth depends on session).
	srv := burrow.NewServer(
		&session.App{},
		&csrf.App{},
		authApp,
		bootstrap.New(),
		&healthcheck.App{},
		notes.New(),
		admin.New(admin.DefaultLayout()),
		staticfiles.New(emptyFS),
	)

	// Wire admin renderer for auth admin pages (users, invites).
	authApp.SetAdminRenderer(auth.DefaultAdminRenderer())

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
