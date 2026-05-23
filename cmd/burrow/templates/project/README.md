# __ProjectName__

<a href="https://github.com/__GitUser__/__ProjectName__/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/__GitUser__/__ProjectName__/ci.yml?branch=main&label=CI&style=for-the-badge" alt="CI"></a>
<a href="https://github.com/__GitUser__/__ProjectName__/releases"><img src="https://img.shields.io/github/v/release/__GitUser__/__ProjectName__?style=for-the-badge" alt="Release"></a>
<a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/__GitUser__/__ProjectName__?style=for-the-badge" alt="Go Version"></a>
<a href="https://goreportcard.com/report/github.com/__GitUser__/__ProjectName__"><img src="https://goreportcard.com/badge/github.com/__GitUser__/__ProjectName__?style=for-the-badge" alt="Go Report Card"></a>
<a href="/LICENSE"><img src="https://img.shields.io/github/license/__GitUser__/__ProjectName__?style=for-the-badge" alt="License"></a>

__ProjectDescription__

Built on [Burrow](https://github.com/oliverandrich/burrow) (modular Go web framework), Tailwind v4 (utility-first CSS, dark mode via `prefers-color-scheme`), [htmx](https://htmx.org/) (server-driven interactivity), SQLite via [Den](https://github.com/oliverandrich/den) (pure Go, no CGO).

## Quick Start

```bash
# Bootstrap the project (mise install + go mod tidy + git hooks)
mise run setup

# Run the development server with live reload (auto-generates .env on first run)
mise run dev
```

Server: <http://localhost:8080>.

## Requirements

- [mise](https://mise.jdx.dev/) — installs every other tool pinned in `.mise.toml`

## Development

`mise tasks` lists every task. The dev loop:

| Task | What it does |
|---|---|
| `mise run dev` | `burrow dev`: watches the project, rebuilds Tailwind CSS, restarts the Go app on every save — sequentially, in one process. |
| `mise run test` | `go test ./...` via tparse |
| `mise run lint` | golangci-lint |
| `mise run fmt` | gofmt + goimports |
| `mise run vuln` | govulncheck |

On first `mise run dev` an `.env` file is generated with persistent `SESSION_HASH_KEY` and `CSRF_KEY` (mode 0600) so sessions and CSRF tokens survive reloads. The file is gitignored. Edit it to add your own dev-time variables — `burrow dev` reads the whole file as a dotenv and injects the keys into the app's environment.

## Project Structure

```
├── cmd/
│   └── <name>/                  # Server entry point
│       └── main.go              # NewServer + cli wiring
├── internal/
│   └── app/                     # Shell app: layout, homepage, CSS bundle (Pattern B)
│       ├── app.go               # HasTemplates, HasStaticFiles, NavItems, Routes
│       ├── static/              # Tailwind output (app.min.css, gitignored)
│       └── templates/
│           ├── app/
│           │   ├── layout.html  # Site layout with navbar, alerts, htmx
│           │   └── icons.html   # Inline-SVG icon defines
│           ├── error/           # Tailwind-styled overrides for 403/404/500/...
│           │   └── errors.html
│           └── pages/
│               └── home.html
├── tailwind.css                 # Tailwind entrypoint (imports source list)
├── Dockerfile                   # Multi-arch image (linux/amd64 + linux/arm64)
├── go.mod                       # Pinned burrow + Den versions
├── .mise.toml                   # Tool pins + task runner
├── .golangci.yml                # Linter config
└── .goreleaser.yaml             # Release config (archives + Docker image)
```

`internal/app/` follows the Pattern B project layout from Burrow's [Tailwind guide](https://burrow.readthedocs.io/en/latest/guide/tailwind/): the shell app owns the layout templates, the compiled Tailwind CSS, and the project-level icon defines, all served under the `/static/app/` URL prefix.

## Releases

Push a `v*` tag to trigger the release workflow:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The workflow runs `mise run release` (= `goreleaser release --clean`) which in one go:

1. Builds binaries for linux / darwin / windows × amd64 / arm64
2. Packages them as `tar.gz` (Linux) / `zip` (macOS, Windows) + `checksums.txt`
3. Builds a multi-arch Docker image (`linux/amd64` + `linux/arm64`)
4. Uploads the archives to GitHub Releases (with auto-generated notes grouped by `feat:` / `fix:`)
5. Pushes the image to GitHub Container Registry as `ghcr.io/<user>/<project>:v<version>` and `:latest`

Local alternatives:

| Task | When |
|---|---|
| `mise run release-snapshot` | Sanity-check the goreleaser config without publishing or building Docker |
| `mise run release-no-docker` | Cut a binaries-only release (e.g. when ghcr auth is missing) |
| `mise run release` | Same as the CI workflow — needs an authenticated `gh` + Docker login to ghcr.io |

Run the published Docker image:

```bash
mkdir -p data && sudo chown 65532:65532 data  # distroless nonroot UID
docker run --rm -p 8080:8080 \
    -v $PWD/data:/data \
    -e DATABASE_DSN=sqlite:////data/app.db \
    ghcr.io/<user>/<project>:latest
```

## License

MIT — see [LICENSE](LICENSE).
