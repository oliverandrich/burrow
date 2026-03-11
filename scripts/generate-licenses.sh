#!/usr/bin/env bash
# Generates THIRD_PARTY_LICENSES.md by combining the static stub with
# dynamically resolved Go module dependencies via go-licenses.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
STUB="$SCRIPT_DIR/licenses-stub.md"
OUTPUT="$ROOT_DIR/THIRD_PARTY_LICENSES.md"
MODULE="github.com/oliverandrich/burrow"

if ! command -v go-licenses &>/dev/null; then
    echo "Error: go-licenses not found. Install with:"
    echo "  go install github.com/google/go-licenses@latest"
    exit 1
fi

# Start with the static stub.
cp "$STUB" "$OUTPUT"

# Append the Go module dependencies header.
cat >> "$OUTPUT" <<'HEADER'

## Go Module Dependencies

Generated with [google/go-licenses](https://github.com/google/go-licenses).

| Module | License | URL |
|--------|---------|-----|
HEADER

# Run go-licenses, filter out our own modules, apply overrides for modules
# where go-licenses cannot detect the license, and format as markdown table.
go-licenses report ./... 2>/dev/null \
    | sort \
    | grep -v "^${MODULE}" \
    | sed 's|^modernc.org/mathutil,Unknown,Unknown$|modernc.org/mathutil,https://gitlab.com/cznic/mathutil/-/blob/master/LICENSE,BSD-3-Clause|' \
    | while IFS=',' read -r mod url license; do
        printf '| %s | %s | [LICENSE](%s) |\n' "$mod" "$license" "$url"
    done >> "$OUTPUT"

echo "Generated $OUTPUT"
