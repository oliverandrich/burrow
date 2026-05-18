// Command burrow is the Burrow framework's command-line companion. It
// scaffolds new projects and apps, and wraps the Tailwind v4 standalone
// CLI with auto-discovered template sources.
//
// Sub-commands:
//
//	burrow new <dir> --module <path>     scaffold a new burrow project
//	burrow tailwind <args...>            run tailwindcss with auto @source listing
//
// The `new` sub-command targets project authors and is typically invoked
// outside any existing Go module via the installed binary:
//
//	go install github.com/oliverandrich/burrow/cmd/burrow@latest
//	burrow new myapp --module github.com/me/myapp
//	cd myapp && go run .
//
// The `tailwind` sub-command targets in-project build pipelines and is
// usually invoked through the `tool` directive in the project's go.mod:
//
//	go tool burrow tailwind -i tailwind.css -o static/app.min.css --minify
package main

import (
	"context"
	"embed"
	"log"
	"os"
	"sync"

	"github.com/urfave/cli/v3"
)

//go:embed all:templates
var templatesFS embed.FS

func main() {
	cmd := &cli.Command{
		Name:    "burrow",
		Usage:   "Burrow framework CLI — scaffold projects and build assets",
		Version: burrowVersion(),
		Commands: []*cli.Command{
			newCommand(),
			generateCommand(),
			tailwindCommand(),
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

// burrowVersion reports the version of the burrow module this binary was
// built from, cached for the lifetime of the process. Falls back to
// "dev" when no build info or git tag is available — in that case the
// generated `go.mod` will not be valid and the user must override via
// `go mod edit -replace` or install a tagged release of burrow.
var burrowVersion = sync.OnceValue(func() string {
	if v := readBurrowVersion(); v != "" {
		return v
	}
	return "dev"
})
