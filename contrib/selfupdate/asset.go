package selfupdate

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// defaultAssetPattern matches the archive name produced by the
// scaffold's `.goreleaser.yaml`:
//
//	{{ .ProjectName }}-{{ .Version }}-{{ .Os }}-{{ x86_64|arm64 }}.{{ .Ext }}
//
// Version has any leading "v" stripped to align with goreleaser's
// `{{ .Version }}` which is the tag without the prefix.
const defaultAssetPattern = `{{ .Name }}-{{ .Version }}-{{ .OS }}-{{ .ArchAlias }}.{{ .Ext }}`

// assetVars are the template variables exposed to asset-pattern
// templates. ArchAlias is the goreleaser-style alias (amd64 →
// x86_64), Arch is the raw runtime.GOARCH for users who want the
// bare form.
type assetVars struct {
	Name      string // project / binary name
	Version   string // semver without leading "v"
	OS        string // runtime.GOOS
	Arch      string // runtime.GOARCH
	ArchAlias string // goreleaser alias (amd64→x86_64, arm64→arm64, ...)
	Ext       string // archive extension (tar.gz on linux, zip on darwin/windows)
}

func resolveAssetName(pattern string, vars assetVars) (string, error) {
	if vars.ArchAlias == "" {
		vars.ArchAlias = archAlias(vars.Arch)
	}
	if vars.Ext == "" {
		vars.Ext = archiveExt(vars.OS)
	}
	vars.Version = strings.TrimPrefix(vars.Version, "v")

	tmpl, err := template.New("asset").Option("missingkey=error").Parse(pattern)
	if err != nil {
		return "", fmt.Errorf("selfupdate: parse asset pattern: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("selfupdate: render asset pattern: %w", err)
	}
	return buf.String(), nil
}

// archAlias maps Go's GOARCH to the alias goreleaser uses by default
// for archive names: amd64 → x86_64, arm64 → arm64, others
// passthrough.
func archAlias(arch string) string {
	switch arch {
	case "amd64":
		return "x86_64"
	default:
		return arch
	}
}

// archiveExt returns the archive extension the burrow scaffold's
// goreleaser config produces per OS (tar.gz on Linux, zip on darwin
// + windows). Used to populate `{{ .Ext }}` in the default asset
// pattern — extraction time uses [archiveFormatFromName] instead
// because [WithAssetMatcher] callers may pull archives that don't
// follow this convention.
func archiveExt(os string) string {
	switch os {
	case "darwin", "windows":
		return "zip"
	default:
		return "tar.gz"
	}
}

// archiveFormatFromName derives the archive type from an asset
// filename's suffix. Returns an error on an unsupported extension.
// This is what the extraction path uses — it must work for
// [WithAssetMatcher] callers whose archives may not match the
// burrow-scaffold conventions encoded in [archiveExt].
func archiveFormatFromName(name string) (string, error) {
	switch {
	case strings.HasSuffix(name, ".tar.gz"), strings.HasSuffix(name, ".tgz"):
		return "tar.gz", nil
	case strings.HasSuffix(name, ".zip"):
		return "zip", nil
	default:
		return "", fmt.Errorf("selfupdate: unsupported archive format for %q (want .tar.gz, .tgz, or .zip)", name)
	}
}

// binaryName returns the binary's filename inside the archive
// (with .exe on Windows).
func binaryName(project, os string) string {
	if os == "windows" {
		return project + ".exe"
	}
	return project
}
