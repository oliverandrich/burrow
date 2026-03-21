# Building Releases

Burrow apps compile to a single static binary with all assets embedded. This page covers building binaries locally and publishing releases via GitHub Actions.

## Prerequisites

- [GoReleaser](https://goreleaser.com/install/) — for cross-compiled builds and GitHub releases
- [just](https://github.com/casey/just) — task runner (optional, for the convenience commands)

## Local Builds

### Quick build (current platform)

The simplest way to build a binary for your current platform:

```bash
go build -o myapp ./cmd/myapp
```

To inject the version number at build time:

```bash
go build -ldflags="-X 'main.version=1.2.3'" -o myapp ./cmd/myapp
```

Or derive it from Git:

```bash
go build -ldflags="-X 'main.version=$(git describe --tags --always --dirty)'" -o myapp ./cmd/myapp
```

!!! tip
    The [project template](https://github.com/oliverandrich/go-burrow-template) includes a `just run` recipe that injects the version automatically.

### Production build (optimized)

For production binaries, strip debug info and use `-trimpath` for reproducible builds:

```bash
CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w -X 'main.version=1.2.3'" \
    -o myapp ./cmd/myapp
```

| Flag | Purpose |
|------|---------|
| `CGO_ENABLED=0` | Pure Go build, no C dependencies — required for static binaries and cross-compilation |
| `-trimpath` | Removes local file paths from the binary for reproducible builds |
| `-s -w` | Strips debug symbols and DWARF info, reducing binary size by ~30% |
| `-X main.version=...` | Sets the `version` variable in `main` at link time |

### Cross-compilation with GoReleaser

Build binaries for all target platforms without publishing a release:

```bash
goreleaser build --snapshot --clean
```

This creates binaries in `dist/` for every OS/architecture combination defined in `.goreleaser.yaml`. The `--snapshot` flag skips the version tag requirement.

## GoReleaser Configuration

The [project template](https://github.com/oliverandrich/go-burrow-template) ships a complete `.goreleaser.yaml`. Here is the essential structure:

```yaml
version: 2
project_name: myapp

builds:
  - main: ./cmd/myapp
    binary: myapp
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X main.version={{.Version}}

archives:
  - name_template: >-
      {{ .ProjectName }}-
      {{- .Version }}-
      {{- .Os }}-
      {{- if eq .Arch "amd64" }}x86_64
      {{- else if eq .Arch "arm64" }}arm64
      {{- else }}{{ .Arch }}{{ end }}
    formats:
      - tar.gz
    format_overrides:
      - goos: windows
        formats:
          - zip
      - goos: darwin
        formats:
          - zip

checksum:
  name_template: checksums.txt
  algorithm: sha256

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
  groups:
    - title: Features
      regexp: "^feat"
    - title: Bug Fixes
      regexp: "^fix"
    - title: Others
      order: 999

release:
  prerelease: auto
```

### Key decisions

**`CGO_ENABLED=0`** is essential. Burrow uses `modernc.org/sqlite` (pure Go SQLite) specifically so that `CGO_ENABLED=0` works. This enables true static binaries and hassle-free cross-compilation — no C toolchain needed for any target platform.

**`-X main.version={{.Version}}`** injects the Git tag as the version. Your `main.go` should have:

```go
var version = "dev"
```

GoReleaser replaces `{{.Version}}` with the tag (e.g., `1.2.3` from tag `v1.2.3`).

### Optional: Homebrew tap

The template includes a Homebrew Cask section for macOS distribution:

```yaml
homebrew_casks:
  - name: myapp
    homepage: https://github.com/user/myapp
    description: My Burrow application
    binaries:
      - myapp
    directory: Casks
    repository:
      owner: user
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
```

This uses GoReleaser's [Homebrew Cask](https://goreleaser.com/customization/homebrew/) support for macOS binary distribution. If you don't need Homebrew distribution, you can remove this section entirely.

This requires a separate `homebrew-tap` repository and a personal access token with `repo` scope stored as `HOMEBREW_TAP_GITHUB_TOKEN` in your repository secrets.

## Publishing a Release

### Manual release

Tag and push to trigger the release:

```bash
git tag v1.0.0
git push origin v1.0.0
```

Or build and publish locally:

```bash
export GITHUB_TOKEN=ghp_...
goreleaser release --clean
```

### Automated release with GitHub Actions

The template includes a release workflow at `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**How it works:**

1. Push a tag: `git tag v1.0.0 && git push origin v1.0.0`
2. GitHub Actions triggers the workflow
3. GoReleaser builds binaries for all platforms, generates checksums, creates the changelog
4. A GitHub Release is created with all artifacts attached

**Important settings:**

| Setting | Why |
|---------|-----|
| `fetch-depth: 0` | GoReleaser needs the full Git history to generate the changelog |
| `go-version-file: go.mod` | Uses the same Go version as your project |
| `permissions: contents: write` | Required to create the GitHub Release |
| `GITHUB_TOKEN` | Automatically provided by GitHub Actions |

### CI workflow

The template also includes a CI workflow at `.github/workflows/ci.yml` that runs on every push and pull request:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Build
        run: go build ./...

      - name: Test
        run: go test ./...

      - name: Lint
        uses: golangci/golangci-lint-action@v7
        with:
          version: latest
```

## Version Injection Pattern

The standard pattern for version injection in a Burrow app:

```go
// version is set via ldflags at build time.
var version = "dev"

func init() {
    // Fall back to Go module version when not built with ldflags
    // (e.g., installed via `go install`).
    if version == "dev" {
        if info, ok := debug.ReadBuildInfo(); ok &&
            info.Main.Version != "" &&
            info.Main.Version != "(devel)" {
            version = info.Main.Version
        }
    }
}
```

This gives three levels of version information:

1. **GoReleaser / ldflags**: `v1.2.3` — the Git tag, set during release builds
2. **`go install`**: module version from Go's build info (e.g., `v1.2.3`)
3. **Local dev build**: falls back to `"dev"`

## Quick Reference

```bash
# Development
go run ./cmd/myapp                              # run directly
just run                                        # run with version injection

# Local builds
go build -o myapp ./cmd/myapp                   # simple build
goreleaser build --snapshot --clean             # cross-compile all platforms

# Release
git tag v1.0.0 && git push origin v1.0.0       # trigger CI release
goreleaser release --clean                      # manual release (needs GITHUB_TOKEN)
```
