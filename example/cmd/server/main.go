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
	admintpl "codeberg.org/oliverandrich/burrow/contrib/admin/templates"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	authtpl "codeberg.org/oliverandrich/burrow/contrib/auth/templates"
	"codeberg.org/oliverandrich/burrow/contrib/bootstrap"
	"codeberg.org/oliverandrich/burrow/contrib/csrf"
	"codeberg.org/oliverandrich/burrow/contrib/healthcheck"
	"codeberg.org/oliverandrich/burrow/contrib/i18n"
	"codeberg.org/oliverandrich/burrow/contrib/messages"
	"codeberg.org/oliverandrich/burrow/contrib/session"
	"codeberg.org/oliverandrich/burrow/contrib/staticfiles"
	"codeberg.org/oliverandrich/burrow/example/internal/layout"
	"codeberg.org/oliverandrich/burrow/example/internal/notes"
	"codeberg.org/oliverandrich/burrow/example/internal/pages"
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
	authApp := auth.New(authtpl.DefaultRenderer())

	// Create the staticfiles app (walks FS to compute content hashes).
	staticApp, err := staticfiles.New(emptyFS)
	if err != nil {
		log.Fatal(err)
	}

	// Create the server with apps in dependency order.
	// Session must come before auth (auth depends on session).
	srv := burrow.NewServer(
		session.New(),
		csrf.New(),
		i18n.New(),
		messages.New(),
		authApp,
		bootstrap.New(),
		healthcheck.New(),
		pages.New(),
		notes.New(),
		admin.New(admintpl.Layout(), admintpl.DefaultDashboardRenderer()),
		staticApp,
	)

	// Use the app layout with navbar (overrides bare bootstrap layout).
	srv.SetLayout(layout.Layout())

	// Use a minimal layout for public auth pages (login, register, recovery).
	authApp.SetAuthLayout(authtpl.AuthLayout())

	// Show a brand logo above the auth forms.
	authApp.SetLogo(layout.Logo())

	// Wire admin renderer for auth admin pages (users, invites).
	authApp.SetAdminRenderer(authtpl.DefaultAdminRenderer())

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
