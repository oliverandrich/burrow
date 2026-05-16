// Command burrow-purge minifies a CSS file down to only the rules
// referenced by the templates of a Burrow app — pure Go, no node, no
// purgecss-binary.
//
// Designed for Burrow's vendored Litewind: static utility classes, no
// JIT, no arbitrary values, no dynamic class composition. Selectors
// with class components are kept only when every class appears in the
// scanned content; tag-, pseudo-, and at-rule-only rules are kept
// unconditionally (see [Purge] for details).
//
// Typical invocation in an app's build pipeline:
//
//	go tool burrow-purge --out static/app.min.css
//
// Defaults: input CSS is Burrow's vendored Litewind (resolved via
// `go list -m`), content patterns are `templates/**/*.html` plus
// Burrow's own contrib templates (auto-included; suppress with
// `--no-burrow-contribs`).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type stringSliceFlag []string

func (s *stringSliceFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error { *s = append(*s, v); return nil }

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "burrow-purge:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, errOut io.Writer) error {
	fs := flag.NewFlagSet("burrow-purge", flag.ContinueOnError)
	fs.SetOutput(errOut)
	var (
		outPath          string
		cssPath          string
		contentGlobs     stringSliceFlag
		keepClasses      stringSliceFlag
		noBurrowContribs bool
		verbose          bool
	)
	fs.StringVar(&outPath, "out", "", "Output CSS file (required)")
	fs.StringVar(&cssPath, "css", "", "Source CSS path (default: Burrow's vendored Litewind)")
	fs.Var(&contentGlobs, "content", "Content file glob; repeat for multiple (default: 'templates/**/*.html')")
	fs.Var(&keepClasses, "keep", "Class name to force-keep even if not found in content; repeat for multiple")
	fs.BoolVar(&noBurrowContribs, "no-burrow-contribs", false, "Do not auto-include Burrow's contrib templates in the content scan")
	fs.BoolVar(&verbose, "verbose", false, "Print a per-file content scan summary to stderr")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if outPath == "" {
		fs.Usage()
		return errors.New("--out is required")
	}

	if cssPath == "" {
		resolved, err := resolveDefaultCSS(ctx)
		if err != nil {
			return fmt.Errorf("resolve default CSS: %w", err)
		}
		cssPath = resolved
	}

	if len(contentGlobs) == 0 {
		contentGlobs = stringSliceFlag{"templates/**/*.html"}
	}

	contentFiles, err := resolveContent(ctx, contentGlobs, !noBurrowContribs)
	if err != nil {
		return fmt.Errorf("resolve content: %w", err)
	}
	if verbose {
		_, _ = fmt.Fprintf(errOut, "scanning %d content file(s)\n", len(contentFiles))
	}

	classes, err := scanContent(contentFiles, verbose, errOut)
	if err != nil {
		return fmt.Errorf("scan content: %w", err)
	}
	if verbose {
		_, _ = fmt.Fprintf(errOut, "extracted %d distinct class(es)\n", len(classes))
	}

	cssBytes, err := os.ReadFile(cssPath) //nolint:gosec // CLI tool: caller controls the path.
	if err != nil {
		return fmt.Errorf("read css %q: %w", cssPath, err)
	}

	out, err := Purge(cssBytes, classes, Options{Keep: []string(keepClasses)})
	if err != nil {
		return fmt.Errorf("purge: %w", err)
	}

	if err := os.WriteFile(outPath, out, 0o644); err != nil { //nolint:gosec // CSS is a public asset; 0o644 is the expected mode.
		return fmt.Errorf("write %q: %w", outPath, err)
	}
	if verbose {
		_, _ = fmt.Fprintf(errOut, "wrote %s (%d bytes)\n", outPath, len(out))
	}
	return nil
}

// resolveDefaultCSS finds Burrow's vendored Litewind by resolving the
// module path of github.com/oliverandrich/burrow via `go list -m` and
// appending the embedded asset path.
func resolveDefaultCSS(ctx context.Context) (string, error) {
	dir, err := burrowModuleDir(ctx)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "contrib", "litewind", "static", "litewind.min.css"), nil
}

// resolveContent expands the user-supplied globs to a deduplicated file
// list. When autoIncludeContribs is true, Burrow's own contrib
// templates are added to the scan.
func resolveContent(ctx context.Context, globs []string, autoIncludeContribs bool) ([]string, error) {
	seen := make(map[string]struct{})
	add := func(path string) {
		abs, err := filepath.Abs(path)
		if err == nil {
			path = abs
		}
		seen[path] = struct{}{}
	}

	for _, g := range globs {
		matches, err := expandGlob(g)
		if err != nil {
			return nil, fmt.Errorf("glob %q: %w", g, err)
		}
		for _, m := range matches {
			add(m)
		}
	}

	if autoIncludeContribs {
		dir, err := burrowModuleDir(ctx)
		if err != nil {
			return nil, err
		}
		matches, err := walkContribTemplates(dir)
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			add(m)
		}
	}

	files := make([]string, 0, len(seen))
	for f := range seen {
		files = append(files, f)
	}
	sort.Strings(files)
	return files, nil
}

// expandGlob expands a glob pattern. A `**` segment is interpreted as
// "recurse into any directory depth" — filepath.Glob alone does not
// understand `**`, so we walk the tree manually when it appears.
func expandGlob(pattern string) ([]string, error) {
	if !strings.Contains(pattern, "**") {
		return filepath.Glob(pattern)
	}
	// Split at first `**`. Prefix is the directory to walk; suffix is
	// the filename glob to apply at every depth (e.g. `*.html`).
	parts := strings.SplitN(pattern, "**", 2)
	root := strings.TrimSuffix(parts[0], string(filepath.Separator))
	if root == "" {
		root = "."
	}
	tail := strings.TrimPrefix(parts[1], string(filepath.Separator))
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip unreadable subtrees, don't abort
		}
		if d.IsDir() {
			return nil
		}
		matched, mErr := filepath.Match(tail, filepath.Base(path))
		if mErr == nil && matched {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// walkContribTemplates returns every template file under burrowDir/contrib.
func walkContribTemplates(burrowDir string) ([]string, error) {
	root := filepath.Join(burrowDir, "contrib")
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip unreadable subtrees, don't abort
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".html") {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// burrowModuleDir resolves the on-disk path of the Burrow module. Uses
// `go list -m -f '{{.Dir}}'` so it survives module cache moves,
// replace directives, and vendor mode. The result is cached for the
// process — a typical purge run calls this twice (default CSS path and
// contrib-template scan) and each shell-out costs 50-200ms.
func burrowModuleDir(ctx context.Context) (string, error) {
	burrowDirOnce.Do(func() {
		cmd := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Dir}}", "github.com/oliverandrich/burrow")
		out, err := cmd.Output()
		if err != nil {
			burrowDirErr = fmt.Errorf("go list -m github.com/oliverandrich/burrow: %w", err)
			return
		}
		dir := strings.TrimSpace(string(out))
		if dir == "" {
			burrowDirErr = errors.New("go list -m returned empty path")
			return
		}
		burrowDir = dir
	})
	return burrowDir, burrowDirErr
}

var (
	burrowDirOnce sync.Once
	burrowDir     string
	burrowDirErr  error
)

// scanContent reads every content file and merges their extracted
// class sets into one.
func scanContent(files []string, verbose bool, errOut io.Writer) (map[string]bool, error) {
	classes := make(map[string]bool)
	for _, f := range files {
		data, err := os.ReadFile(f) //nolint:gosec // CLI tool: caller-supplied content path.
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", f, err)
		}
		got := ExtractClasses(data)
		if verbose {
			_, _ = fmt.Fprintf(errOut, "  %s: %d class(es)\n", f, len(got))
		}
		for c := range got {
			classes[c] = true
		}
	}
	return classes, nil
}
