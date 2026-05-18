package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

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
		Description: `Scaffold a new burrow project at <dir>. The directory must not
exist or must be empty. When git is on PATH, the destination is
initialized as a git repository. The printed Next-steps adapt to
whether mise is installed.`,
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
			&cli.StringFlag{
				Name:  "author",
				Usage: "copyright holder for LICENSE (default: `git config user.name`, then --git-user)",
			},
		},
		Action: runNew,
	}
}

func runNew(ctx context.Context, cmd *cli.Command) error {
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
	author := cmd.String("author")
	if author == "" {
		author = guessAuthor(gitUser)
	}

	vars := scaffold.Vars{
		"__ProjectName__":        projectName,
		"__ProjectDescription__": cmd.String("description"),
		"__GitUser__":            gitUser,
		"__Author__":             author,
		"__Year__":               strconv.Itoa(time.Now().Year()),
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

	bootstrapGit(ctx, destDir, os.Stderr, defaultHasGit, defaultInitRepo)
	fmt.Print(nextStepsMessage(destDir, projectName, defaultHasMise()))
	return nil
}

func nextStepsMessage(destDir, projectName string, miseAvailable bool) string {
	if miseAvailable {
		return fmt.Sprintf(nextStepsMiseTemplate, destDir)
	}
	return fmt.Sprintf(nextStepsPlainTemplate, destDir, projectName)
}

const nextStepsMiseTemplate = `Project scaffolded at %[1]s.

Next steps:
  cd %[1]s
  mise run setup     # installs tools, fetches deps, generates dev keys, installs git hooks
  mise run dev       # live-reload server

Docs: https://burrow.dev/
`

const nextStepsPlainTemplate = `Project scaffolded at %[1]s.

Next steps:
  cd %[1]s
  go mod tidy
  go run ./cmd/%[2]s

Docs: https://burrow.dev/  (use "go tool burrow tailwind ..." for CSS builds)
`

// bootstrapGit warns instead of aborting — the scaffold already wrote
// files to disk, so a missing git or failed init shouldn't undo the work.
func bootstrapGit(ctx context.Context, destDir string, errW io.Writer, hasGit func() bool, initRepo func(context.Context, string) error) {
	if !hasGit() {
		_, _ = fmt.Fprintln(errW, "warning: git not on PATH — skipping `git init`")
		return
	}
	if err := initRepo(ctx, destDir); err != nil {
		_, _ = fmt.Fprintln(errW, "warning: git init failed:", err)
	}
}

func defaultHasMise() bool {
	_, err := exec.LookPath("mise")
	return err == nil
}

func defaultHasGit() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func defaultInitRepo(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "git", "init", "-q")
	cmd.Dir = dir
	return cmd.Run()
}

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

// guessAuthor falls back to `git config user.name` for a human-readable
// copyright holder, then to gitUser as a last resort. The caller can
// override with --author for CI pipelines or org repos.
func guessAuthor(gitUser string) string {
	return resolveAuthor(gitUser, gitConfigUserName)
}

// resolveAuthor is the testable core of guessAuthor: it consults
// gitNameFn for the canonical name, falling back to gitUser when the
// git config lookup yields nothing.
func resolveAuthor(gitUser string, gitNameFn func() string) string {
	if name := gitNameFn(); name != "" {
		return name
	}
	return gitUser
}

// gitConfigUserName returns the value of `git config user.name`, or
// the empty string when git is unavailable or the config is unset.
func gitConfigUserName() string {
	out, err := exec.CommandContext(context.Background(), "git", "config", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
