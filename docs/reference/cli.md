# `burrow` CLI

The `burrow` binary scaffolds new projects and contrib-style apps, and
wraps the standalone Tailwind v4 CLI with auto-discovered template
sources. It lives at `cmd/burrow` in the framework repo.

## Installation

```bash
go install github.com/oliverandrich/burrow/cmd/burrow@latest
```

The binary lands in `$GOPATH/bin` (or `$GOBIN`). Make sure that
directory is on your `PATH`.

Inside an existing burrow project, you can also reference the binary
as a Go tool via `go.mod`:

```bash
go get -tool github.com/oliverandrich/burrow/cmd/burrow
```

Then invoke as `go tool burrow <sub-command>`. The scaffold generated
by `burrow new` wires this up by default.

## Sub-commands

```
burrow new <dir> --module <path>     scaffold a new burrow project
burrow generate app <name>           scaffold a contrib-style app stub
burrow tailwind <args...>            run tailwindcss with auto @source listing
```

---

## `burrow new`

Scaffold a new burrow project from the bundled template.

### Synopsis

```
burrow new <dir> --module <module-path> [flags]
```

`<dir>` must not exist or must be empty. The scaffold writes a
complete starter project — `cmd/<name>/`, `internal/app/` shell,
`.mise.toml`, `.air.toml`, `.golangci.yml`, `.goreleaser.yaml`,
multi-arch `Dockerfile`, and GitHub Actions CI + release workflows.

### Flags

| Flag | Required | Default | Description |
|---|---|---|---|
| `--module` | yes | — | Target Go module path (e.g. `github.com/me/myapp`) |
| `--description` | no | "" | Project description, used in README and Docker image labels |
| `--git-user` | no | second segment of `--module` | GitHub user or org |
| `--author` | no | `git config user.name`, then `--git-user` | Copyright holder for the LICENSE |

### Burrow version pinning

The generated `go.mod` pins `github.com/oliverandrich/burrow` to the
version that produced it. Resolution order:

1. `runtime/debug.ReadBuildInfo` — set automatically when the binary
   was installed via `go install ...@vX.Y.Z`.
2. `git describe --tags --abbrev=0 --match 'v*'` in the cwd — covers
   `go run ./cmd/burrow` from inside the burrow source tree.

### Auto `git init`

When `git` is on `PATH`, the destination directory is initialized as a
git repository (`git init -q`). The initial commit is left to the user.
When git is missing, a stderr warning is printed and the scaffold
continues without `.git/`.

### Next-steps output

The printed Next-steps adapt to whether `mise` is installed:

```bash
# With mise on PATH (recommended path — the scaffold pins mise tasks)
cd myapp
mise run setup     # installs tools, fetches deps, generates dev keys, installs git hooks
mise run dev       # live-reload server

# Without mise
cd myapp
go mod tidy
go run ./cmd/myapp
```

### Example

```bash
burrow new myapp --module github.com/me/myapp --description "My demo app"
cd myapp
mise run setup
mise run dev
# → http://localhost:8080
```

---

## `burrow generate app`

Scaffold a contrib-style app stub inside an existing project.

### Synopsis

```
burrow generate app <name> [--path <base-dir>]
```

The generated stub has the standard contrib-app shape: `app.go`,
`app_test.go`, and `templates/<name>/index.html`. It registers `GET
/<name>` and renders `<name>/index`.

### Flags

| Flag | Default | Description |
|---|---|---|
| `--path` | `./internal` | Base directory for the app. Output goes to `<path>/<name>` |

### Name validation

App names must be:

- Non-empty
- Start with a lowercase ASCII letter
- Then any mix of lowercase letters, digits, and underscores
- Not a Go keyword (`for`, `package`, etc.)
- Not a predeclared identifier (`string`, `error`, `make`, etc.)

### Host module detection

The next-steps hint reads the cwd's `go.mod` via
`golang.org/x/mod/modfile.ModulePath` to print the full Go import path
for the new app (`"github.com/me/myapp/internal/<name>"`). When no
`go.mod` is found, it falls back to a placeholder.

### Example

```bash
cd myapp
burrow generate app notes
# → internal/notes/{app.go, app_test.go, templates/notes/index.html}

burrow generate app marketing --path ./apps
# → apps/marketing/...
```

Wire the new app up by adding it to the server in `cmd/<project>/main.go`:

```go
srv := burrow.NewServer(
    // ... existing apps ...
    notes.New(),
)
```

---

## `burrow tailwind`

Invoke the standalone Tailwind v4 CLI with a pre-generated
`@source` listing that covers every contrib and every project app.

### Synopsis

```
burrow tailwind <args...>
```

All arguments are forwarded verbatim to `tailwindcss`. On every
invocation, `burrow tailwind` writes `.tailwind/sources.css` next to
the input CSS (or in the cwd) with `@source "<absolute path>";` lines
for:

1. Every `<burrow>/contrib/<app>/templates/` directory (resolved via
   `go list -m`).
2. The project's `./templates/` if it exists (flat layout).
3. Every `./internal/<app>/templates/` directory (structured layout).

### Requirements

The Tailwind v4 standalone CLI (`tailwindcss`) must be on `PATH`. mise
users get it via the pin in `.mise.toml`:

```toml
"github:tailwindlabs/tailwindcss" = "4"
```

### Example

```bash
go tool burrow tailwind -i tailwind.css -o internal/app/static/app.min.css --minify
go tool burrow tailwind -i tailwind.css -o internal/app/static/app.min.css --watch
```

See [Tailwind CSS](../guide/tailwind.md) for the full setup pattern.

### Deprecated alternative

The standalone `cmd/burrow-tailwind` binary still works as a
deprecation shim — it prints a stderr notice and delegates to the
`burrow tailwind` sub-command. Removal is scheduled for v0.22. See
the migration callout at the top of [Tailwind CSS](../guide/tailwind.md).
