#!/usr/bin/env bash
# Generates llms-full.txt by concatenating all documentation markdown files
# in a logical order. Output goes to docs/llms-full.txt.
set -euo pipefail

DOCS_DIR="docs"
OUTPUT="$DOCS_DIR/llms-full.txt"

# Files in reading order: core docs first, then guides, contrib, reference, tutorial.
files=(
    index.md
    getting-started/quickstart.md
    getting-started/installation.md
    getting-started/project-structure.md
    guide/creating-an-app.md
    guide/routing.md
    guide/database.md
    guide/migrations.md
    guide/configuration.md
    guide/layouts.md
    guide/navigation.md
    guide/validation.md
    guide/pagination.md
    guide/inter-app-communication.md
    guide/tls.md
    guide/deployment.md
    reference/interfaces.md
    reference/core-functions.md
    reference/server.md
    reference/configuration.md
    reference/template-functions.md
    reference/context-helpers.md
    contrib/auth.md
    contrib/session.md
    contrib/csrf.md
    contrib/admin.md
    contrib/i18n.md
    contrib/bootstrap.md
    contrib/bsicons.md
    contrib/htmx.md
    contrib/jobs.md
    contrib/uploads.md
    contrib/messages.md
    contrib/ratelimit.md
    contrib/staticfiles.md
    contrib/healthcheck.md
    contrib/authmail.md
    tutorial/index.md
    tutorial/part1.md
    tutorial/part2.md
    tutorial/part3.md
    tutorial/part4.md
    tutorial/part5.md
    tutorial/part6.md
    tutorial/part7.md
    contributing.md
    changelog.md
)

{
    echo "# Burrow — Full Documentation"
    echo ""
    echo "> Complete documentation for Burrow, a modular Go web framework. Built on Chi, Bun/SQLite, and html/template."
    echo ""

    for file in "${files[@]}"; do
        path="$DOCS_DIR/$file"
        if [[ -f "$path" ]]; then
            echo "---"
            echo ""
            cat "$path"
            echo ""
        fi
    done
} > "$OUTPUT"

wc -l < "$OUTPUT" | xargs printf "Generated %s (%s lines)\n" "$OUTPUT"
