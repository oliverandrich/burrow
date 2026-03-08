# Contributing

Thank you for considering contributing to Burrow! This guide explains how to set up a development environment and submit changes.

## Prerequisites

- **Go 1.25+** — [go.dev/dl](https://go.dev/dl/)
- **just** — task runner ([github.com/casey/just](https://github.com/casey/just))
- **pre-commit** — git hook manager ([pre-commit.com](https://pre-commit.com/#install))

Additional tools (linter, test formatter, etc.) are checked automatically — see [Development Setup](#development-setup) below.

## Development Setup

```bash
# Clone the repository
git clone https://github.com/oliverandrich/burrow.git
cd burrow

# Check that all required tools are installed
just setup

# Install git hooks
pre-commit install
```

`just setup` verifies that Go, golangci-lint, tparse, goimports, govulncheck, and pre-commit are available. It prints install instructions for anything missing.

## Development Workflow

The `justfile` provides all common tasks:

| Command | Description |
|---|---|
| `just test` | Run all tests |
| `just lint` | Run golangci-lint |
| `just fmt` | Format all Go files (gofmt + goimports) |
| `just coverage` | Run tests with coverage report |
| `just tidy` | Tidy module dependencies |
| `just docs` | Serve documentation locally |

A typical workflow:

```bash
# Make your changes, then:
just fmt
just lint
just test
```

The pre-commit hooks run the linter, vulnerability checker, tests, and `go mod tidy` automatically on each commit.

## Code Style

- **Linting** — golangci-lint with the project's [`.golangci.yml`](https://github.com/oliverandrich/burrow/blob/main/.golangci.yml) config. All code must pass `just lint` without warnings.
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
4. **Ensure all checks pass** — `just fmt && just lint && just test`.
5. **Open a pull request** against `main` with a clear description of *what* and *why*.

### What we look for in reviews

- Tests for new functionality and bug fixes
- Clean commit history with conventional commit messages
- No unrelated changes mixed into the PR
- Code passes the linter without new warnings

## License

By contributing, you agree that your contributions will be licensed under the [European Union Public Licence v1.2 (EUPL-1.2)](license.md).
