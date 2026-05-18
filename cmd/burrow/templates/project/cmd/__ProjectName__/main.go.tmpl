// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"embed"
	"log"
	"log/slog"
	"os"
	"runtime/debug"

	"github.com/lmittmann/tint"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/csrf"
	"github.com/oliverandrich/burrow/contrib/healthcheck"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/oliverandrich/burrow/contrib/staticfiles"
	_ "github.com/oliverandrich/den/backend/sqlite" // register sqlite:// scheme
	"github.com/oliverandrich/go-burrow-template/internal/app"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

// version is set via ldflags at build time.
var version = "dev"

// emptyFS is an empty filesystem for the framework's root staticfiles app.
// The project's stylesheet (Tailwind output) is contributed by internal/app
// via its own HasStaticFiles under the "app" prefix.
var emptyFS embed.FS

func init() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
}

func main() {
	// Console logging via lmittmann/tint — colorized in a TTY, plain text
	// when stdout is piped (log files, journald, docker logs etc.) so
	// production log aggregators get clean output.
	slog.SetDefault(slog.New(tint.NewHandler(os.Stdout, &tint.Options{
		Level:   slog.LevelDebug,
		NoColor: !term.IsTerminal(int(os.Stdout.Fd())),
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
		messages.New(),
		htmx.New(),
		app.New(),
	)

	srv.SetLayout("app/layout")

	cmd := &cli.Command{
		Name:     "__ProjectName__",
		Usage:    "__ProjectDescription__",
		Version:  version,
		Flags:    srv.Flags(nil),
		Action:   srv.Run,
		Commands: srv.CLICommands(),
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
