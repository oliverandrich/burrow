// Package tailwind drives the standalone Tailwind v4 CLI with a
// pre-generated `@source` listing that points at every template
// directory in the project — Burrow's contribs plus the project's own
// templates.
//
// It is the engine behind both `burrow tailwind` (the canonical
// invocation) and the legacy `burrow-tailwind` standalone tool (kept
// as a deprecation shim that calls [Run] directly).
//
// On every invocation [Run] writes `.tailwind/sources.css` next to
// the input CSS file passed via `-i` (or relative to the working
// directory when `-i` is not given) with `@source "<absolute path>";`
// lines for:
//
//   - Every `<burrow>/contrib/<app>/templates/` directory in the Burrow
//     module the project depends on (resolved via `go list -m`).
//   - Every `<project>/internal/<app>/templates/` directory in the
//     project's own Go module (resolved via `go env GOMOD`).
//   - The project's `<project>/templates/` if it exists (flat layout).
//
// The project's `tailwind.css` is expected to `@import "./.tailwind/sources.css"`.
// For the conventional Burrow layouts (flat `./templates` or
// `./internal/<app>/templates/`) no extra `@source` lines are needed.
//
// Tailwind v4's standalone Rust CLI must be on PATH. mise users can
// pin it via `.mise.toml` (see `docs/guide/tailwind.md`).
package tailwind

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	burrowModule          = "github.com/oliverandrich/burrow"
	defaultSourcesOutPath = ".tailwind/sources.css"
)

// Run invokes the standalone Tailwind CLI with args forwarded verbatim,
// after writing the auto-generated `@source` listing to
// `.tailwind/sources.css` next to the -i input (or in the cwd).
//
// stdin/stdout/stderr are inherited from the parent process so users
// see Tailwind's output live.
func Run(ctx context.Context, args []string) error {
	burrowDir, err := burrowModuleDir(ctx)
	if err != nil {
		return fmt.Errorf("resolve burrow module dir: %w", err)
	}

	// Project root is best-effort: outside a Go module we still want
	// to source Burrow's contribs.
	projectRoot, _ := projectRootDir(ctx)

	dirs := collectTemplateDirs(burrowDir, projectRoot)
	sourcesPath := sourcesOutPath(args)
	if err := writeSources(sourcesPath, dirs); err != nil {
		return fmt.Errorf("write %s: %w", sourcesPath, err)
	}

	tw, err := exec.LookPath("tailwindcss")
	if err != nil {
		return fmt.Errorf("tailwindcss not found on PATH — install it via `mise install` or download a release from https://github.com/tailwindlabs/tailwindcss/releases")
	}

	cmd := exec.CommandContext(ctx, tw, args...) //nolint:gosec // CLI tool: passes user args verbatim by design.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// collectTemplateDirs returns the absolute paths of every template
// directory that should be scanned by Tailwind, sorted for
// deterministic output. The set is the union of:
//
//   - Burrow contribs at `<burrowDir>/contrib/<app>/templates/`.
//   - Project root templates at `<projectRoot>/templates/` (flat layout).
//   - Project internal apps at `<projectRoot>/internal/<app>/templates/`.
//
// projectRoot may be empty (when run outside a Go module); in that
// case only the burrow contribs are returned.
func collectTemplateDirs(burrowDir, projectRoot string) []string {
	dirs := findTemplateSubdirs(filepath.Join(burrowDir, "contrib"))

	if projectRoot != "" {
		if root := filepath.Join(projectRoot, "templates"); isDir(root) {
			dirs = append(dirs, root)
		}
		dirs = append(dirs, findTemplateSubdirs(filepath.Join(projectRoot, "internal"))...)
	}

	sort.Strings(dirs)
	return dirs
}

// findTemplateSubdirs returns absolute paths of `<root>/<entry>/templates`
// for every directory entry in root that has a `templates` subdirectory.
// Missing root returns an empty slice — both "no such directory" and
// "directory exists but has no template-bearing subdirs" are normal.
func findTemplateSubdirs(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		tmpl := filepath.Join(root, e.Name(), "templates")
		if isDir(tmpl) {
			out = append(out, tmpl)
		}
	}
	return out
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// sourcesOutPath picks the destination for the generated sources.css.
// When tailwindcss is invoked with `-i <path>` (or its long form
// `--input <path>`), the file is written next to that input so the
// `@import "./.tailwind/sources.css"` line inside tailwind.css resolves
// correctly regardless of cwd. Otherwise it falls back to the
// cwd-relative default.
func sourcesOutPath(args []string) string {
	for i, a := range args {
		var input string
		switch {
		case (a == "-i" || a == "--input") && i+1 < len(args):
			input = args[i+1]
		case strings.HasPrefix(a, "-i="):
			input = strings.TrimPrefix(a, "-i=")
		case strings.HasPrefix(a, "--input="):
			input = strings.TrimPrefix(a, "--input=")
		}
		if input != "" {
			return filepath.Join(filepath.Dir(input), defaultSourcesOutPath)
		}
	}
	return defaultSourcesOutPath
}

// writeSources writes a CSS file at outPath that contains one
// `@source "<absolute path>";` line per entry in dirs.
func writeSources(outPath string, dirs []string) error {
	var b strings.Builder
	b.WriteString("/* Auto-generated by `burrow tailwind` — do not edit; regenerated on every invocation. */\n")
	for _, d := range dirs {
		fmt.Fprintf(&b, "@source %q;\n", d)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil { //nolint:gosec // Output is a public CSS asset; 0o755 is the conventional mode for project directories.
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(outPath), err)
	}
	return os.WriteFile(outPath, []byte(b.String()), 0o644) //nolint:gosec // CSS asset, 0o644 is the right mode.
}

// burrowModuleDir resolves the on-disk path of the Burrow module via
// `go list -m -f '{{.Dir}}' <module>`. Honours module cache layout,
// replace directives, and vendor mode.
func burrowModuleDir(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Dir}}", burrowModule).Output()
	if err != nil {
		return "", fmt.Errorf("go list -m %s: %w", burrowModule, err)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", errors.New("go list -m returned empty path")
	}
	return dir, nil
}

// projectRootDir resolves the directory containing the current
// project's `go.mod` via `go env GOMOD`. Returns an error when the
// command's working directory is outside any Go module.
func projectRootDir(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "go", "env", "GOMOD").Output()
	if err != nil {
		return "", fmt.Errorf("go env GOMOD: %w", err)
	}
	gomod := strings.TrimSpace(string(out))
	if gomod == "" || gomod == os.DevNull {
		return "", errors.New("not in a Go module")
	}
	return filepath.Dir(gomod), nil
}
