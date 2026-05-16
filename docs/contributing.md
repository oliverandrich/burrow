# Contributing

Thank you for considering contributing to Burrow! This guide explains how to set up a development environment and submit changes.

## Prerequisites

- **[mise](https://mise.jdx.dev/)** — version manager + task runner. One install gives you the pinned Go toolchain, all dev tools, and the `mise run <task>` commands.
- **pre-commit** — git hook manager ([pre-commit.com](https://pre-commit.com/#install))

## Development Setup

```bash
# Clone the repository
git clone https://github.com/oliverandrich/burrow.git
cd burrow

# Install pinned tools (Go, golangci-lint, tparse, goimports, govulncheck, ...)
mise install

# Verify the environment
mise run setup

# Install git hooks
pre-commit install
```

`mise install` reads `.mise.toml` and installs every pinned tool at the exact version Burrow targets. `mise run setup` then checks that each tool is reachable on PATH and prints a summary.

## Development Workflow

`.mise.toml` and `mise-tasks/` define all common tasks:

| Command | Description |
|---|---|
| `mise run test` | Run all tests |
| `mise run lint` | Run golangci-lint |
| `mise run fmt` | Format all Go files (gofmt + goimports) |
| `mise run coverage` | Run tests with coverage report |
| `mise run tidy` | Tidy module dependencies |
| `mise run docs` | Serve documentation locally |
| `mise tasks` | List all available tasks |

`mise run <task>` can be shortened to `mise <task>` when the task name doesn't collide with a mise builtin.

A typical workflow:

```bash
# Make your changes, then:
mise run fmt
mise run lint
mise run test
```

The pre-commit hooks run the linter, vulnerability checker, tests, and `go mod tidy` automatically on each commit.

## Code Style

- **Linting** — golangci-lint with the project's [`.golangci.yml`](https://github.com/oliverandrich/burrow/blob/main/.golangci.yml) config. All code must pass `mise run lint` without warnings.
- **Testing** — use [testify](https://github.com/stretchr/testify) (`assert`, `require`) for all tests.
- **Commit messages** — follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`, `perf:`).

## Project Structure

```
burrow.go       # App interface, AppConfig struct
server.go       # Server (boot sequence, middleware, graceful shutdown)
config.go       # Config with CLI/ENV/TOML sourcing
registry.go     # Registry for app management
contrib/        # Reusable apps (auth, session, csrf, i18n, admin, ...)
example/        # Example applications
docs/           # Documentation (this site)
```

Each contrib app follows a standard layout — see the [Creating an App](guide/creating-an-app.md) guide for the conventions.

## Submitting Changes

1. **Fork** the repository on GitHub.
2. **Create a branch** from `main` with a descriptive name (e.g. `feat/widget-support`, `fix/session-race`).
3. **Make focused commits** — each commit should represent one logical change.
4. **Ensure all checks pass** — `mise run fmt && mise run lint && mise run test`.
5. **Open a pull request** against `main` with a clear description of *what* and *why*.

### What we look for in reviews

- Tests for new functionality and bug fixes
- Clean commit history with conventional commit messages
- No unrelated changes mixed into the PR
- Code passes the linter without new warnings

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](license.md).
