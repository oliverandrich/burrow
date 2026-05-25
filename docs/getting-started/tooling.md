# Tooling

Burrow ships a CLI for scaffolding and a separate Claude Code plugin
with framework-aware agents. Both are optional — Burrow works fine
without them.

## CLI — `burrow`

The `burrow` binary scaffolds new projects and contrib-style apps, and
wraps the standalone Tailwind CLI with auto-discovered template
sources. It lives at `cmd/burrow` in the framework repo and installs
in one line:

```bash
go install github.com/oliverandrich/burrow/cmd/burrow@latest
```

### Scaffold a new project

```bash
burrow new myapp --module github.com/you/myapp
cd myapp
mise run setup     # installs tools, fetches deps, generates dev keys, installs git hooks
mise run dev       # live-reload server
```

When `mise` isn't installed, the scaffold prints a `go mod tidy && go run ./cmd/myapp` fallback instead.

The scaffold ships:

- `cmd/<name>/main.go` wiring a typical contrib stack (`session`,
  `csrf`, `staticfiles`, `healthcheck`, `messages`, `htmx`) plus a
  Tailwind-classed shell app providing the homepage and layout
  overrides.
- `.mise.toml` pinning the Go toolchain, Tailwind v4,
  `golangci-lint`, `tparse`, `goimports`, `govulncheck`, `goreleaser`,
  and `pre-commit`.
- `mise run dev` calls `burrow dev`, which watches the project and on
  every change rebuilds Tailwind CSS then restarts the Go app —
  sequentially, in one process.
- `.golangci.yml`, `.goreleaser.yaml`, multi-arch `Dockerfile`, and
  GitHub Actions CI workflow (with a tag-only release stage).
- A pre-commit config that calls the mise-installed tools (no extra
  Go toolchains).

For a file-by-file walkthrough, the full `mise` task reference, the
dev/release loops, and what the scaffold deliberately omits, see
[Scaffold](scaffold.md).

The generated `go.mod` auto-pins to the burrow version that produced
it, via `runtime/debug.ReadBuildInfo`.

#### Flags

| Flag | Required | Default | Purpose |
|---|---|---|---|
| `--module` | yes | — | Target Go module path (e.g. `github.com/me/myapp`) |
| `--description` | no | "" | Project description for README and Docker labels |
| `--git-user` | no | second segment of `--module` | GitHub user or org |
| `--author` | no | `git config user.name`, then `--git-user` | Copyright holder for LICENSE |

### Scaffold a contrib-style app

Inside an existing project:

```bash
burrow generate app notes
```

Produces `internal/notes/{app.go, app_test.go, templates/notes/index.html}`.
The stub registers `GET /notes` and renders `notes/index`. The next-steps
hint prints the full import path, derived from the host module's `go.mod`.

Override the output base via `--path`:

```bash
burrow generate app marketing --path ./apps
# → ./apps/marketing/...
```

App names must be valid Go identifiers — lowercase ASCII letter +
letters/digits/underscores, not a Go keyword or predeclared identifier.

### Tailwind CSS builds

`burrow tailwind` invokes the standalone Tailwind v4 CLI with an
auto-generated `@source` listing that covers every contrib and every
project app:

```bash
go tool burrow tailwind -i tailwind.css -o internal/app/static/app.min.css --minify
```

See [Tailwind CSS](../guide/tailwind.md) for the full setup. All
arguments are forwarded verbatim to `tailwindcss`.

## Claude Code Plugin — `burrow-claude-plugin`

[github.com/oliverandrich/burrow-claude-plugin](https://github.com/oliverandrich/burrow-claude-plugin)

A [Claude Code](https://claude.ai/code) plugin with agents and
commands specialized for Burrow. The agents carry framework knowledge
(app lifecycle, optional interfaces, configuration conventions, Den
repository patterns, template namespacing, testing style) so they make
convention-consistent changes without being told the same rules every
session.

### Commands

Interactive workflows that run in your main conversation:

| Command | Purpose |
|---|---|
| `/burrow-feature-dev` | End-to-end feature development — discovery, exploration, clarifying questions, TDD implementation, review, docs |
| `/burrow-review` | Interactive code review against framework conventions; launches the reviewer agent and helps fix findings |
| `/burrow-architect` | Architecture design session; produces a blueprint and persists it as a bean |
| `/burrow-setup` | Configures a project for use with the plugin |

### Agents

Specialized subagents with focused prompts and tool restrictions. Used
by the commands above or invoked directly via `@agent-name`:

| Agent | Role |
|---|---|
| `burrow-dev` | Full-stack feature developer: research → plan → TDD → verify → document |
| `burrow-architect` | Read-only architecture advisor; designs implementation blueprints |
| `burrow-reviewer` | Read-only code reviewer; reports convention violations by severity |
| `burrow-den-expert` | Deep Den knowledge — consulted by other agents for query patterns, document modeling, backend differences |
| `burrow-user` | Simulates a first-time Burrow developer reading the docs; flags where it gets stuck |

### Installation

One-time marketplace setup:

```
/plugin marketplace add oliverandrich/burrow-claude-plugin
```

Then install the plugin:

```
# available in all your projects
/plugin install burrow-claude-plugin --scope user

# or just for the current project
/plugin install burrow-claude-plugin --scope project
```

The plugin reads the Burrow version from your project's `go.mod` at
runtime, so the same installation works across projects pinned to
different Burrow versions.
