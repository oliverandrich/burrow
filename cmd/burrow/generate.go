package main

import (
	"context"
	"fmt"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/oliverandrich/burrow/internal/scaffold"
	"github.com/urfave/cli/v3"
	"golang.org/x/mod/modfile"
)

func generateCommand() *cli.Command {
	return &cli.Command{
		Name:  "generate",
		Usage: "Scaffold a new app inside an existing burrow project",
		Commands: []*cli.Command{
			generateAppCommand(),
		},
	}
}

func generateAppCommand() *cli.Command {
	return &cli.Command{
		Name:      "app",
		Usage:     "Scaffold a new contrib-style app stub",
		ArgsUsage: "<name>",
		Description: `Generate a contrib-style app at <path>/<name>/. The output directory
` + "`<path>/<name>` must not exist or must be empty." + `

The generated stub is a minimal burrow app — App struct, New(), Name(),
TemplateFS(), and one route at /<name> that renders templates/<name>/index.html.
Wire it up by adding <name>.New() to the burrow.NewServer(...) call in
your project's cmd/<project>/main.go.`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "base directory for the app (default ./internal)",
				Value: "./internal",
			},
		},
		Action: runGenerateApp,
	}
}

func runGenerateApp(_ context.Context, cmd *cli.Command) error {
	if cmd.NArg() != 1 {
		return fmt.Errorf("burrow generate app: exactly one <name> argument is required")
	}
	appName := cmd.Args().First()
	if err := validateAppName(appName); err != nil {
		return err
	}

	destDir := filepath.Join(cmd.String("path"), appName)

	vars := scaffold.Vars{
		"__AppName__": appName,
	}

	src, err := fs.Sub(templatesFS, "templates/app")
	if err != nil {
		return fmt.Errorf("locate app templates: %w", err)
	}

	if err := scaffold.Render(src, destDir, vars, nil); err != nil {
		return err
	}

	importPath := importPathHint(destDir)
	fmt.Printf(generateAppNextStepsTemplate, destDir, importPath, appName, appName)
	return nil
}

const generateAppNextStepsTemplate = `App stub generated at %s.

Next steps:
  - Import in your server's main.go:    %q
  - Register it on the server:          burrow.NewServer(..., %s.New())
  - Visit http://localhost:8080/%s after starting the server.
`

// importPathHint computes the Go import path for the generated app
// stub, derived from the host project's go.mod. Returns a placeholder
// when no go.mod is found (e.g. running from a non-module dir).
func importPathHint(destDir string) string {
	module := readHostModule(".")
	if module == "" {
		return "<your-module>/" + filepath.ToSlash(destDir)
	}
	return module + "/" + filepath.ToSlash(destDir)
}

// readHostModule extracts the `module <path>` declaration from the
// go.mod file in dir via the canonical modfile parser. Returns the
// empty string when dir has no go.mod or the file lacks a module
// directive — callers branch on the empty case.
func readHostModule(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod")) //nolint:gosec // dir is the host project root resolved by burrow's own logic, not untrusted input
	if err != nil {
		return ""
	}
	return modfile.ModulePath(data)
}

// validateAppName rejects names that wouldn't make a valid Go package
// or that would conflict with reserved words. The first character must
// be a lowercase letter; the rest letters, digits, or underscores.
func validateAppName(name string) error {
	if name == "" {
		return fmt.Errorf("app name is required")
	}
	if token.Lookup(name).IsKeyword() || types.Universe.Lookup(name) != nil {
		return fmt.Errorf("%q is a Go reserved word and can't be used as a package name", name)
	}
	first := rune(name[0])
	if !isLowerLetter(first) {
		if isLetter(first) {
			return fmt.Errorf("app name must start with a lowercase letter, got %q", name)
		}
		return fmt.Errorf("app name must start with a letter, got %q", name)
	}
	for _, r := range name[1:] {
		if !isLowerLetter(r) && !isDigit(r) && r != '_' {
			return fmt.Errorf("app name may only contain lowercase letters, digits, and underscores, got %q", name)
		}
	}
	return nil
}

func isLowerLetter(r rune) bool { return r >= 'a' && r <= 'z' }
func isLetter(r rune) bool      { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }
func isDigit(r rune) bool       { return r >= '0' && r <= '9' }
