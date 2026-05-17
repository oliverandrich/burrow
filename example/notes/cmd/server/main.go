// Command server demonstrates how to build an application
// using the burrow framework with contrib apps. Following Pattern B
// (see docs/guide/tailwind.md), the project's layout templates and
// compiled Tailwind CSS live in `internal/app/`; this main.go just
// wires apps together.
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
	"github.com/oliverandrich/burrow/contrib/csrf"
	"github.com/oliverandrich/burrow/contrib/healthcheck"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/humanize"
	"github.com/oliverandrich/burrow/contrib/jobs"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/oliverandrich/burrow/contrib/staticfiles"
	"github.com/oliverandrich/burrow/example/notes/internal/app"
	"github.com/oliverandrich/burrow/example/notes/internal/notes"
	_ "github.com/oliverandrich/den/backend/sqlite" // register sqlite:// scheme
	"github.com/urfave/cli/v3"
)

// version is set via ldflags at build time.
var version = "dev"

// emptyFS holds no user-level static assets — the framework's root
// static-files app needs *some* fs.FS but the actual CSS bundle is
// embedded by `internal/app` and served via its `HasStaticFiles`
// implementation under `/static/app/...`.
var emptyFS embed.FS

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	staticApp, err := staticfiles.New(emptyFS)
	if err != nil {
		log.Fatal(err)
	}

	srv := burrow.NewServer(
		session.New(),
		csrf.New(),
		staticApp,
		healthcheck.New(),
		jobs.New(),
		app.New(),
		messages.New(),
		auth.New(
			auth.WithLogoComponent(app.Logo()),
		),
		htmx.New(),
		humanize.New(),
		notes.New(),
		admin.New(),
	)

	srv.SetLayout("app/layout")

	cmd := &cli.Command{
		Name:     "example",
		Usage:    "Example application using the burrow framework",
		Version:  version,
		Flags:    srv.Flags(nil),
		Action:   srv.Run,
		Commands: srv.CLICommands(),
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
