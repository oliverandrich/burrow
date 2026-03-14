# Default recipe: list available commands
default:
    @just --list

# Run all tests
test *args:
    go test -race -json {{args}} ./... | tparse

# Run linter
lint:
    golangci-lint run ./...

# Format all Go files
fmt:
    gofmt -w .
    goimports -w .

# Run tests with coverage (excludes generated code via go-ignore-cov)
coverage:
    #!/usr/bin/env bash
    set -euo pipefail
    go test -json -coverprofile=coverage.out ./... > test.json
    go-ignore-cov --file coverage.out --exclude-globs "**/*_generated.go"
    # Patch coverage percentages in JSON for packages containing generated files
    cp test.json test.patched.json
    for dir in $(find . -name '*_generated.go' -type f | xargs -I{} dirname {} | sort -u); do
        pkg=$(go list "./$dir")
        pct=$(go tool cover -func=coverage.out | grep "^${pkg}/" | grep -c '100.0%' | xargs -I{} echo "100.0%") || true
        # Calculate actual percentage: covered / total statements
        total=$(grep "^${pkg}/" coverage.out | wc -l | tr -d ' ')
        covered=$(grep "^${pkg}/" coverage.out | grep -v ' 0$' | wc -l | tr -d ' ')
        if [ "$total" -gt 0 ]; then
            pct=$(echo "scale=1; $covered * 100 / $total" | bc)
        else
            pct="0.0"
        fi
        sed -i '' "s|\\(\"Package\":\"${pkg}\".*coverage: \\)[0-9.]*%|\\1${pct}%|g" test.patched.json
    done
    tparse -file=test.patched.json
    rm -f test.json test.patched.json
    go tool cover -html=coverage.out -o coverage.html
    echo "Coverage report: coverage.html"

# Check that all required dev tools are installed
setup:
    #!/usr/bin/env bash
    set -euo pipefail
    ok=true
    check() {
        if command -v "$1" &>/dev/null; then
            printf "  %-20s %s\n" "$1" "$(command -v "$1")"
        else
            printf "  %-20s MISSING — %s\n" "$1" "$2"
            ok=false
        fi
    }
    echo "Checking dev tools:"
    check go              "https://go.dev/dl/"
    check golangci-lint   "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
    check tparse          "go install github.com/mfridman/tparse@latest"
    check goimports       "go install golang.org/x/tools/cmd/goimports@latest"
    check govulncheck     "go install golang.org/x/vuln/cmd/govulncheck@latest"
    check go-ignore-cov   "go install github.com/quantumcycle/go-ignore-cov@latest"
    check go-licenses     "go install github.com/google/go-licenses@latest"
    check pre-commit      "https://pre-commit.com/#install"
    echo ""
    if $ok; then
        echo "All tools installed."
        echo "Run 'pre-commit install' to set up git hooks."
    else
        echo "Some tools are missing. Install them and re-run 'just setup'."
        exit 1
    fi

# Tidy module dependencies
tidy:
    go mod tidy

# Run the hello world example
example-hello *args:
    go run ./example/hello {{args}}

# Run the notes example application
example-notes *args:
    go run ./example/notes/cmd/server {{args}}

# Regenerate THIRD_PARTY_LICENSES.md from go-licenses
licenses:
    ./scripts/generate-licenses.sh

# Serve documentation locally
docs:
    ./scripts/generate-licenses.sh
    cp CHANGELOG.md docs/changelog.md
    cp THIRD_PARTY_LICENSES.md docs/third-party-licenses.md
    uv run --with zensical zensical serve -a localhost:3000

# Build documentation
docs-build:
    ./scripts/generate-licenses.sh
    cp CHANGELOG.md docs/changelog.md
    cp THIRD_PARTY_LICENSES.md docs/third-party-licenses.md
    ./scripts/generate-llms-full.sh
    uv run --with zensical zensical build

# Update Bootstrap Icons SVG components (downloads latest release)
update-icons version="1.13.1":
    #!/usr/bin/env bash
    set -euo pipefail
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT
    echo "Downloading Bootstrap Icons v{{version}}..."
    curl -sL "https://github.com/twbs/icons/releases/download/v{{version}}/bootstrap-icons-{{version}}.zip" -o "$tmp/icons.zip"
    unzip -q -o "$tmp/icons.zip" -d "$tmp/extract"
    echo "Generating icon functions..."
    go run ./contrib/bsicons/internal/generate -icons-dir "$tmp/extract/bootstrap-icons-{{version}}" > contrib/bsicons/icons_generated.go
    count=$(grep -c '^func ' contrib/bsicons/icons_generated.go)
    echo "Done — $count icons generated from Bootstrap Icons v{{version}}"

# Update Bootstrap CSS/JS assets (downloads latest release)
update-bootstrap version="5.3.8":
    #!/usr/bin/env bash
    set -euo pipefail
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT
    echo "Downloading Bootstrap v{{version}}..."
    curl -sL "https://github.com/twbs/bootstrap/releases/download/v{{version}}/bootstrap-{{version}}-dist.zip" -o "$tmp/bootstrap.zip"
    unzip -q -o "$tmp/bootstrap.zip" -d "$tmp/extract"
    cp "$tmp/extract/bootstrap-{{version}}-dist/css/bootstrap.min.css" contrib/bootstrap/static/
    cp "$tmp/extract/bootstrap-{{version}}-dist/js/bootstrap.bundle.min.js" contrib/bootstrap/static/
    echo "Done — Bootstrap v{{version}} updated"

# Update htmx (downloads latest release)
update-htmx version="2.0.8":
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Downloading htmx v{{version}}..."
    curl -sL "https://unpkg.com/htmx.org@{{version}}/dist/htmx.min.js" -o contrib/htmx/static/htmx.min.js
    echo "Done — htmx v{{version}} updated"
