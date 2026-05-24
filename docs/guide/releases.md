# Building Releases

Burrow apps compile to a single static binary with all assets embedded. The scaffold ships everything needed to produce cross-platform binaries plus a multi-arch Docker image on every `v*` tag push, via [GoReleaser](https://goreleaser.com/) and a GitHub Actions workflow.

This page covers the framework-level decisions baked into the scaffold (why those choices), the local commands you'll use day-to-day, and how the release pipeline fits together. For the exact config, read your project's own `.goreleaser.yaml`, `.mise.toml`, and `.github/workflows/ci.yml` — they're the source of truth and stay in sync with the scaffold template.

## Why pure-Go static binaries

The scaffold pins `modernc.org/sqlite` (pure Go) and builds with `CGO_ENABLED=0`. The pay-off is real:

- Single static binary per target — no C toolchain on the build host, no shared-library hunt at runtime.
- Cross-compilation to every supported OS/arch in one `goreleaser` invocation. The scaffold ships builds for Linux, macOS, Windows, FreeBSD, and OpenBSD on both amd64 and arm64.
- Docker images can use `gcr.io/distroless/static-debian12:nonroot` — no `libc`, smaller surface, no `glibc-vs-musl` headaches.

## Local builds

### Quick build for your current platform

```bash
go build -o myapp ./cmd/myapp
```

### Install locally with version injection

The scaffold's `mise run install` task drops a stripped, version-stamped binary into `$GOPATH/bin`:

```bash
mise run install
# = mise run css && go install -ldflags="-s -w -X 'main.version=$(git describe ...)'" ./cmd/myapp
```

`mise run install`, `release`, `release-no-docker`, and `release-snapshot` all `depends = ["css"]`, so each first rebuilds the Tailwind bundle via `mise run css` before the Go step. Expect a couple-hundred-millisecond Tailwind invocation in front of every release-path command.

### Release-flag reference

These are the flags goreleaser uses for every release build. Mostly informational — you don't set them directly in day-to-day work, the scaffold's `.goreleaser.yaml` does.

| Flag | Purpose |
|------|---------|
| `CGO_ENABLED=0` | Pure-Go build — required for static binaries and cross-compilation |
| `-trimpath` | Reproducible builds (no local file paths leak in) |
| `-s -w` | Strip debug symbols + DWARF — about 30% smaller binary |
| `-X main.version={{.Version}}` | Inject the Git tag into `main.version` at link time |

### Cross-compile all targets without publishing

```bash
mise run release-snapshot
# = goreleaser release --snapshot --clean --skip=publish,docker
```

Produces archives in `dist/` for every OS/arch combination defined in `.goreleaser.yaml`. The `--snapshot` flag waives the tag requirement so you can sanity-check the pipeline without cutting a release.

## The version-injection pattern

The scaffold's `main.go` follows the same three-level fallback every well-behaved Go CLI uses:

```go
// version is set via ldflags at build time.
var version = "dev"

func init() {
    if version == "dev" {
        if info, ok := debug.ReadBuildInfo(); ok &&
            info.Main.Version != "" &&
            info.Main.Version != "(devel)" {
            version = info.Main.Version
        }
    }
}
```

1. **GoReleaser / `mise run install`**: `v1.2.3` — Git tag, baked in via `-X main.version=...`.
2. **`go install github.com/me/myapp@latest`**: Go's build info supplies the module version.
3. **Local `go run`**: falls back to `"dev"`.

`contrib/selfupdate`'s `WithCurrentVersion(version)` is passed this same variable, so the chain works end-to-end.

## Publishing a release

### Cut a release

```bash
git tag v1.0.0
git push origin v1.0.0
```

That's it. The scaffold's `.github/workflows/ci.yml` has a tag-only `release` job at the bottom; on every `v*` tag push it:

1. Runs the regular `check` + `zizmor` jobs (the same gate that runs on `main` pushes).
2. Once both are green, `mise run release` (= `goreleaser release --clean`) builds every binary, generates `checksums.txt`, renders the changelog from commit messages, and uploads them to a fresh GitHub Release.
3. Builds a multi-arch (`linux/amd64` + `linux/arm64`) Docker image and pushes it to `ghcr.io/<user>/<project>:v<version>` + `:latest`.

The whole thing runs unattended with the workflow's auto-provided `GITHUB_TOKEN`. No personal access token, no extra secrets — `packages: write` permission is enough for the ghcr push because the actor is the tag's pusher.

Tags shaped like `v1.0.0-rc1` or `v1.0.0-beta1` are marked as **prereleases** on GitHub (goreleaser's `release.prerelease: auto` handles this). Override by editing `.goreleaser.yaml` if you want strict-stable semantics.

### Cut a release locally (no CI)

For one-off manual releases from your workstation:

```bash
export GITHUB_TOKEN=ghp_...        # PAT with repo + write:packages scopes
echo "$GITHUB_TOKEN" | docker login ghcr.io -u <github-user> --password-stdin
mise run release
```

Both auth steps are required: the token uploads the GitHub Release, the Docker login lets goreleaser push the image to ghcr.io. Skip the Docker login and the build will fail mid-way through the push.

When you don't have (or don't want) Docker on the host, use the no-Docker variant — still needs the token for the Release upload:

```bash
export GITHUB_TOKEN=ghp_...
mise run release-no-docker
# = goreleaser release --clean --skip=docker
```

For a pure sanity check that produces no uploads and no Docker calls, use `mise run release-snapshot` (see above) instead.

## Optional integrations

The scaffold's `.goreleaser.yaml` covers the common path (cross-platform binaries + checksums + Docker). GoReleaser itself supports a lot more — Homebrew taps, Scoop manifests, nFPM packages (.deb / .rpm), Snap, and others. None of these are wired into the scaffold; if you need them, read [goreleaser's customization docs](https://goreleaser.com/customization/) and add the relevant sections directly to your project's `.goreleaser.yaml`.

## Quick reference

```bash
# Development
mise run dev                                    # live reload (burrow dev)

# Local builds
go build -o myapp ./cmd/myapp                   # simple build
mise run install                                # production-shaped local install
mise run release-snapshot                       # cross-compile every target (no upload)

# Release
git tag v1.0.0 && git push origin v1.0.0        # CI release (full pipeline)
mise run release                                # local manual release (needs GITHUB_TOKEN)
mise run release-no-docker                      # local release without ghcr push
```
