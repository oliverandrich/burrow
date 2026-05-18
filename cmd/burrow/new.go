package main

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/oliverandrich/burrow/internal/scaffold"
	"github.com/urfave/cli/v3"
)

// templateModulePath is the module path used inside the embedded project
// template. Render rewrites it to the user's --module value.
const templateModulePath = "github.com/oliverandrich/go-burrow-template"

func newCommand() *cli.Command {
	return &cli.Command{
		Name:      "new",
		Usage:     "Scaffold a new burrow project",
		ArgsUsage: "<dir>",
		Description: `Scaffold a new burrow project at <dir>. The directory must not exist
or must be empty. After scaffolding, cd into the project and run
` + "`go mod tidy && go run ./cmd/<name>`.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "module",
				Usage:    "target Go module path (e.g. github.com/me/myapp)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "description",
				Usage: "short project description (used in README and Docker labels)",
			},
			&cli.StringFlag{
				Name:  "git-user",
				Usage: "GitHub user or org (default: second segment of --module)",
			},
		},
		Action: runNew,
	}
}

func runNew(_ context.Context, cmd *cli.Command) error {
	if cmd.NArg() != 1 {
		return fmt.Errorf("burrow new: exactly one <dir> argument is required")
	}
	destDir := cmd.Args().First()

	modulePath := cmd.String("module")
	if err := validateModulePath(modulePath); err != nil {
		return err
	}

	projectName := path.Base(modulePath)
	gitUser := cmd.String("git-user")
	if gitUser == "" {
		gitUser = guessGitUser(modulePath)
	}

	vars := scaffold.Vars{
		"__ProjectName__":        projectName,
		"__ProjectDescription__": cmd.String("description"),
		"__GitUser__":            gitUser,
		"__BurrowVersion__":      burrowVersion(),
	}
	rewrite := &scaffold.ModuleRewrite{
		From: templateModulePath,
		To:   modulePath,
	}

	src, err := fs.Sub(templatesFS, "templates/project")
	if err != nil {
		return fmt.Errorf("locate project templates: %w", err)
	}

	if err := scaffold.Render(src, destDir, vars, rewrite); err != nil {
		return err
	}

	fmt.Printf(nextStepsTemplate, destDir, destDir, projectName)
	return nil
}

const nextStepsTemplate = `Project scaffolded at %s.

Next steps:
  cd %s
  go mod tidy
  go run ./cmd/%s

Docs: https://burrow.dev/  (use "go tool burrow tailwind ..." for CSS builds)
`

// validateModulePath rejects empty or obviously malformed module paths.
// Full validation is delegated to `go mod tidy`.
func validateModulePath(p string) error {
	if p == "" {
		return fmt.Errorf("--module is required")
	}
	if strings.Contains(p, " ") {
		return fmt.Errorf("--module must not contain spaces")
	}
	if !strings.Contains(p, "/") {
		return fmt.Errorf("--module must contain at least one slash (e.g. github.com/me/myapp)")
	}
	return nil
}

// guessGitUser picks the second path segment as the most common GitHub
// org/user shape (github.com/<user>/<repo>). For non-github hosts the
// caller can override with --git-user.
func guessGitUser(modulePath string) string {
	parts := strings.Split(modulePath, "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}
