// Package scaffold renders embedded file trees into a destination directory,
// applying placeholder substitution and (optionally) a Go module path rewrite.
//
// It is the engine behind the `burrow new` and `burrow generate app`
// CLI sub-commands. Templates live in [io/fs.FS] sources (typically
// [embed.FS]) and are written to a destination directory on disk.
package scaffold

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Vars maps placeholder tokens (e.g. "__ProjectName__") to their replacement values.
// Substitution is applied to both file contents and file paths.
type Vars map[string]string

// ModuleRewrite specifies a from→to Go module path rewrite. The rewrite is
// applied as a plain string substitution in files ending in `.go` and in
// files named `go.mod` — never in other file types.
type ModuleRewrite struct {
	From, To string
}

// Render walks srcFS and writes each entry to destDir, applying placeholder
// substitution from vars to both file paths and contents, and applying
// moduleRewrite (if non-nil) to `.go` and `go.mod` files.
//
// destDir must not exist or must be empty — Render refuses to write into a
// directory that already contains files.
func Render(srcFS fs.FS, destDir string, vars Vars, moduleRewrite *ModuleRewrite) error {
	if err := ensureEmptyDir(destDir); err != nil {
		return err
	}

	replacer := newReplacer(vars)

	return fs.WalkDir(srcFS, ".", func(srcPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		content, err := fs.ReadFile(srcFS, srcPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", srcPath, err)
		}

		destPath := filepath.Join(destDir, stripTmplSuffix(replacer.Replace(srcPath)))
		content = substituteContent(content, replacer, srcPath, moduleRewrite)

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil { //nolint:gosec // scaffolded project dirs are user-readable by convention
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(destPath), err)
		}
		if err := os.WriteFile(destPath, content, 0o644); err != nil { //nolint:gosec // scaffolded source files are world-readable by convention (matches `go mod init`)
			return fmt.Errorf("write %s: %w", destPath, err)
		}
		return nil
	})
}

func ensureEmptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return os.MkdirAll(dir, 0o755) //nolint:gosec // scaffolded project dirs are user-readable by convention
		}
		return fmt.Errorf("read destination %s: %w", dir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("destination %s is not empty", dir)
	}
	return nil
}

// newReplacer builds a single-pass replacer over all placeholder→value
// pairs in vars. nil/empty vars yields a no-op replacer.
func newReplacer(vars Vars) *strings.Replacer {
	pairs := make([]string, 0, 2*len(vars))
	for k, v := range vars {
		pairs = append(pairs, k, v)
	}
	return strings.NewReplacer(pairs...)
}

func substituteContent(content []byte, replacer *strings.Replacer, srcPath string, rewrite *ModuleRewrite) []byte {
	s := replacer.Replace(string(content))
	if rewrite != nil && isGoSource(srcPath) {
		s = strings.ReplaceAll(s, rewrite.From, rewrite.To)
	}
	return []byte(s)
}

// isGoSource reports whether p is Go source code for the purpose of
// applying [ModuleRewrite]. Both bare `.go`/`go.mod` and their `.tmpl`
// counterparts qualify — the `.tmpl` suffix is used in embedded
// templates to hide files from the Go toolchain (see [stripTmplSuffix]).
func isGoSource(p string) bool {
	base := filepath.Base(p)
	if base == "go.mod" || base == "go.mod.tmpl" {
		return true
	}
	ext := filepath.Ext(p)
	if ext == ".go" {
		return true
	}
	return ext == ".tmpl" && filepath.Ext(strings.TrimSuffix(p, ext)) == ".go"
}

// stripTmplSuffix strips a trailing `.tmpl` from the filename so that
// embedded files like `go.mod.tmpl` and `app.go.tmpl` (named with
// `.tmpl` to keep `//go:embed` from treating them as Go source or a
// nested module) render as `go.mod` and `app.go` respectively.
func stripTmplSuffix(p string) string {
	if rest, ok := strings.CutSuffix(p, ".tmpl"); ok {
		return rest
	}
	return p
}
