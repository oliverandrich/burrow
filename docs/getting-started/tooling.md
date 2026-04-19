# Tooling

Two companion projects speed up day-to-day work with Burrow: a project
template that scaffolds a production-ready starter, and a Claude Code
plugin with agents that know the framework inside out.

Both are optional. Burrow works fine without them.

## Project Template — `go-burrow-template`

[github.com/oliverandrich/go-burrow-template](https://github.com/oliverandrich/go-burrow-template)

A starter template that gives you a running Burrow application in one
command. It ships with:

- `cmd/<name>/main.go` wiring up a typical contrib stack (`session`,
  `csrf`, `staticfiles`, `healthcheck`, `messages`, `htmx`,
  `bootstrap`) with a `pages` app providing the homepage and layout
  overrides.
- A `justfile` with recipes for `run`, `run-once`, `test`, `coverage`,
  `lint`, `fmt`, `tidy`, `install`, and `setup` (checks your dev
  tools).
- Live reload via [air](https://github.com/air-verse/air): `just run`
  rebuilds and restarts on every `.go` / `.html` change. On first run
  a `.dev-keys` file is generated with persistent `SESSION_HASH_KEY`
  and `CSRF_KEY` so sessions and CSRF tokens survive reloads.
- `.golangci.yml`, `.goreleaser.yaml`, and a GitHub Actions CI
  workflow — matching the conventions used in the framework itself.

### Using it

The template is driven by [gohatch](https://github.com/oliverandrich/gohatch),
which replaces placeholders (`__ProjectName__`, `__GitUser__`,
`__ProjectDescription__`) during scaffolding:

```bash
gohatch github.com/oliverandrich/go-burrow-template github.com/you/your-app
cd your-app
just setup   # verify tools
just run     # start the dev server with live reload
```

If you prefer to copy the repo directly and rename the placeholders
yourself, that works too — gohatch is just a convenience wrapper.

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
