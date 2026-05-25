// Package dev implements the `burrow dev` integrated development
// server: a file watcher that restarts the application on source
// changes and (optionally) runs the Tailwind v4 standalone CLI in
// `--watch` mode alongside it.
//
// The package is dev-tooling only; it is never linked into burrow-built
// binaries unless they depend on `cmd/burrow` for `go tool` purposes.
package dev

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Config drives a single run of the dev server. Most fields can be
// populated by [Discover]; flags on the CLI override individual values.
type Config struct {
	// ProjectRoot is the project's module root (the directory containing go.mod).
	ProjectRoot string

	// AppPath is the Go package path passed to `go run`, e.g. "./cmd/myapp".
	AppPath string

	// CSSIn / CSSOut are the Tailwind input/output paths (relative to
	// ProjectRoot). Both empty means no Tailwind co-watcher.
	CSSIn  string
	CSSOut string

	// WatchExts is the set of file extensions (lowercase, leading dot)
	// that trigger a restart, e.g. {".go", ".html", ".css", ".toml",
	// ".yml", ".yaml"}. TOML and YAML are included so changes to
	// translation bundles and config files are picked up without an
	// explicit `--watch-exts` override.
	WatchExts []string

	// ExcludeDirs is the set of directory names (no path separators)
	// or repo-relative paths to skip during the project walk. Names
	// match anywhere in the tree (".git" matches every .git dir);
	// path entries (containing a separator) match only at that exact
	// location relative to ProjectRoot.
	ExcludeDirs []string

	// Debounce is the quiet window after the first file event before a
	// restart is triggered. Coalesces editor save-rename storms.
	Debounce time.Duration

	// KillTimeout is how long SIGTERM is given before SIGKILL.
	KillTimeout time.Duration

	// EnvFile, when non-empty, is parsed via godotenv and the
	// resulting KEY=VALUE pairs are injected into the child process
	// environment. Missing file is not an error — the dev server
	// auto-creates it with conventional defaults (see [EnsureEnvFile]).
	EnvFile string

	// Stdout / Stderr are where the dev server writes its own log
	// lines (children always inherit os.Stdout / os.Stderr).
	Stdout io.Writer
	Stderr io.Writer
}

// Default values applied by Discover when fields are unset.
const (
	defaultDebounce    = 300 * time.Millisecond
	defaultKillTimeout = 500 * time.Millisecond
	defaultEnvFile     = ".env"
	defaultCSSOutFile  = "app.min.css"
)

var (
	defaultWatchExts   = []string{".go", ".html", ".css", ".toml", ".yml", ".yaml"}
	defaultExcludeDirs = []string{".git", ".beans", "node_modules", "tmp", "testdata", ".tailwind"}
)

// ProjectRoot resolves the directory containing the current project's
// go.mod via `go env GOMOD`. Returns an error when the working
// directory is outside any Go module.
func ProjectRoot(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "go", "env", "GOMOD").Output()
	if err != nil {
		return "", fmt.Errorf("dev: go env GOMOD: %w", err)
	}
	gomod := strings.TrimSpace(string(out))
	if gomod == "" || gomod == os.DevNull {
		return "", errors.New("dev: not inside a Go module")
	}
	return filepath.Dir(gomod), nil
}

// Discover populates a Config from the conventional burrow project
// layout under projectRoot:
//
//   - AppPath = "./cmd/<name>" when exactly one cmd/<name>/main.go exists.
//   - CSSIn / CSSOut are inferred when tailwind.css exists at the root
//     and exactly one internal/<app>/static/ directory exists; in any
//     other case Tailwind discovery silently no-ops (both stay empty)
//     and the user must supply --css-in / --css-out explicitly.
//   - All other fields receive their default values. ExcludeDirs is
//     derived from CSSOut by [applyDefaults] *after* CLI overrides are
//     in place, so a `--css-out` flag override correctly excludes the
//     chosen directory.
func Discover(projectRoot string) (Config, error) {
	if err := refuseBurrowSelf(projectRoot); err != nil {
		return Config{}, err
	}

	appPath, err := discoverAppPath(projectRoot)
	if err != nil {
		return Config{}, err
	}

	cssIn, cssOut := discoverTailwindPaths(projectRoot)

	return Config{
		ProjectRoot: projectRoot,
		AppPath:     appPath,
		CSSIn:       cssIn,
		CSSOut:      cssOut,
		WatchExts:   append([]string{}, defaultWatchExts...),
		ExcludeDirs: append([]string{}, defaultExcludeDirs...),
		Debounce:    defaultDebounce,
		KillTimeout: defaultKillTimeout,
		EnvFile:     defaultEnvFile,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	}, nil
}

// burrowModulePath identifies the burrow framework's own go.mod, so
// Discover can refuse to run dev against the framework itself
// (`go tool burrow dev` from the burrow checkout would otherwise
// auto-pick the burrow CLI as "the app" and confuse maintainers).
const burrowModulePath = "github.com/oliverandrich/burrow"

// refuseBurrowSelf returns an error when projectRoot is the burrow
// framework's own checkout. Detected by reading <projectRoot>/go.mod
// and matching the module directive against burrowModulePath. Missing
// or unreadable go.mod is non-fatal — Discover falls through to its
// regular path.
func refuseBurrowSelf(projectRoot string) error {
	data, err := os.ReadFile(filepath.Join(projectRoot, "go.mod")) //nolint:gosec // projectRoot is from `go env GOMOD` (or a CLI flag); reading its go.mod is the whole point.
	if err != nil {
		return nil
	}
	for line := range strings.Lines(string(data)) {
		trimmed := strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(trimmed, "module ")
		if !ok {
			continue
		}
		if strings.TrimSpace(rest) == burrowModulePath {
			return fmt.Errorf("dev: refusing to run `burrow dev` against the burrow framework itself (module %q); cd into a scaffolded project first", burrowModulePath)
		}
		break
	}
	return nil
}

// discoverAppPath finds the single cmd/<name>/main.go under projectRoot.
// Returns an error when zero or more than one such entry-point exists.
func discoverAppPath(projectRoot string) (string, error) {
	cmdDir := filepath.Join(projectRoot, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return "", errors.New("no cmd/<name>/main.go found: " + cmdDir + " does not exist or is not readable")
	}

	var candidates []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(cmdDir, e.Name(), "main.go")); statErr != nil {
			continue
		}
		candidates = append(candidates, e.Name())
	}
	sort.Strings(candidates)

	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("no cmd/<name>/main.go found under %s", cmdDir)
	case 1:
		return "./" + filepath.ToSlash(filepath.Join("cmd", candidates[0])), nil
	default:
		return "", fmt.Errorf("multiple cmd/<name>/main.go entries found (%v); pass --app to pick one", candidates)
	}
}

// discoverTailwindPaths returns ("tailwind.css", "internal/<app>/static/app.min.css")
// when both the conventional input file and exactly one
// internal/<app>/static/ directory are present. Otherwise returns
// empty strings — Tailwind co-watching is silently disabled and the
// user must opt in via flags.
func discoverTailwindPaths(projectRoot string) (string, string) {
	if _, err := os.Stat(filepath.Join(projectRoot, "tailwind.css")); err != nil {
		return "", ""
	}

	internal := filepath.Join(projectRoot, "internal")
	entries, err := os.ReadDir(internal)
	if err != nil {
		return "", ""
	}

	var staticDirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		staticRel := filepath.Join("internal", e.Name(), "static")
		if info, statErr := os.Stat(filepath.Join(projectRoot, staticRel)); statErr == nil && info.IsDir() {
			staticDirs = append(staticDirs, staticRel)
		}
	}

	if len(staticDirs) != 1 {
		return "", ""
	}
	return "tailwind.css", filepath.Join(staticDirs[0], defaultCSSOutFile)
}
