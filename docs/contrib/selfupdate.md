# Selfupdate

Adds an `update` sub-command to a burrow binary that downloads the latest release for the running OS/arch from GitHub, Codeberg, or any Forgejo instance and atomically replaces the binary in place.

**Package:** `github.com/oliverandrich/burrow/contrib/selfupdate`

**Depends on:** none — uses anonymous HTTPS to the public Releases API. No tokens required.

## Scope

| | |
|---|---|
| **Sources** | GitHub, Codeberg, any Forgejo instance (same Gitea-shaped REST API). |
| **Verification** | HTTPS + SHA256 against the release's `checksums.txt`. No signatures in v1. |
| **Auth** | None. Public repos only. |
| **Atomic swap + rollback** | via [`minio/selfupdate`](https://github.com/minio/selfupdate). |
| **Platforms** | Linux is the tested target. macOS works as a side effect of POSIX file semantics. Windows is best-effort and relies on `minio/selfupdate`'s locked-running-binary handling. |

## Setup

Add to your server wiring in `cmd/<app>/main.go`:

```go
import (
    "github.com/oliverandrich/burrow"
    "github.com/oliverandrich/burrow/contrib/selfupdate"
)

var version = "dev" // set by goreleaser via -ldflags="-X main.version=..."

func main() {
    srv := burrow.NewServer(
        // ... other apps ...
        selfupdate.New(
            selfupdate.WithRepo("me", "myapp"),
            selfupdate.WithHost("github.com"),
            selfupdate.WithCurrentVersion(version),
        ),
    )
    srv.Run()
}
```

`WithRepo`, `WithHost`, and `WithCurrentVersion` are required; everything else is optional.

## CLI

```text
myapp update              # apply latest update
myapp update --check      # report whether an update is available
myapp update --to v1.2.3  # pin to a specific release (also: rollback)
```

The command is always non-interactive — it just does it. There is no prompt.

## Asset layout (defaults match the burrow scaffold)

The default asset-name template matches what the burrow scaffold's `.goreleaser.yaml` produces:

```text
{{ .Name }}-{{ .Version }}-{{ .OS }}-{{ .ArchAlias }}.{{ .Ext }}
```

So a release at tag `v1.2.3` would expose:

- `myapp-1.2.3-linux-x86_64.tar.gz`
- `myapp-1.2.3-darwin-arm64.zip`
- `myapp-1.2.3-windows-x86_64.zip`
- `checksums.txt`

The selfupdate flow:

1. Fetches `/repos/<owner>/<repo>/releases/latest` (or `/releases/tags/<tag>` with `--to`).
2. Compares the release's `tag_name` against `WithCurrentVersion` via [`golang.org/x/mod/semver`](https://pkg.go.dev/golang.org/x/mod/semver).
3. Downloads the matching asset for the running OS/arch and the `checksums.txt`.
4. Verifies the archive against the recorded SHA256.
5. Extracts the binary (`tar.gz` or `zip`).
6. Atomically replaces the running binary via `minio/selfupdate`.

If your project uses a different asset layout, override the template:

```go
selfupdate.New(
    selfupdate.WithRepo("me", "myapp"),
    selfupdate.WithHost("github.com"),
    selfupdate.WithCurrentVersion(version),
    selfupdate.WithAssetPattern(`{{ .Name }}_{{ .Version }}_{{ .OS }}_{{ .Arch }}.{{ .Ext }}`),
)
```

Available template variables: `Name`, `Version` (leading `v` stripped), `OS`, `Arch`, `ArchAlias` (`amd64→x86_64`), `Ext` (`tar.gz` on Linux, `zip` on macOS/Windows).

For layouts where a single template doesn't fit, swap in a predicate via `WithAssetMatcher`:

```go
import "regexp"

var rel = regexp.MustCompile(`^myapp[-_]v?\d`)

selfupdate.New(
    ...,
    selfupdate.WithAssetMatcher(rel.MatchString),
)
```

The matcher is called with each asset's filename and selects the first one returning `true`. It takes precedence over `WithAssetPattern`.

When the binary inside the archive isn't named after the repo (e.g. repo `go-myapp-server`, binary `myapp`), use `WithBinaryName`:

```go
selfupdate.New(
    selfupdate.WithRepo("me", "go-myapp-server"),
    selfupdate.WithBinaryName("myapp"),
    ...,
)
```

## Configuration

| Flag | Env | TOML | Description |
|---|---|---|---|
| `--update-host` | `UPDATE_HOST` | `selfupdate.host` | Override the SCM host (e.g. point at a mirror). |

Repo coords and current version are compile-time only — they live in your `main.go`.

## Failure modes

| Situation | Behaviour |
|---|---|
| Already on latest | Exit 0, prints `selfupdate: already on latest (vX.Y.Z)`. |
| `--to` pins to the current tag | Exit 0, prints `selfupdate: already on vX.Y.Z`. |
| Asset not found in release | Error naming the expected asset name and the release tag. Original binary untouched. |
| Checksum mismatch | Error, refuses to install. Original binary untouched. The archive is verified *before* the tar/zip parser runs on it. |
| `Permission denied` writing the binary | Error suggests running as the binary's owner (often root for system installs). |
| Release host returns 404 | Error suggests checking `WithRepo` / `WithHost`. |
| Release host returns 403 with `X-RateLimit-Remaining: 0` | Error names the rate-limit-reset timestamp so the user knows when to retry. |
| Concurrent `myapp update` already running | Error: `another update is already running (lock held on ...)` (Linux/macOS only). |
| Current version is not valid semver | Error at action time — `WithCurrentVersion` requires a `vX.Y.Z` (or `vX.Y.Z-rc1`) string. |

## After the update

The atomic swap replaces the on-disk binary. The currently-running process keeps executing the *old* code in memory — for daemons and long-running servers, you must restart the process to pick up the new version. The contrib prints a reminder on success:

```text
selfupdate: installed v1.2.3 — restart this binary to run the new version
```
