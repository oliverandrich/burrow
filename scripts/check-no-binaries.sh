#!/usr/bin/env bash
# Reject staged files that look like compiled executables (Mach-O / ELF / PE).
# Build outputs do not belong in git — keep them out of commits.
set -euo pipefail

violations=()

for f in "$@"; do
    [ -f "$f" ] || continue
    mime=$(file --brief --mime-type -- "$f")
    case "$mime" in
        application/x-mach-binary \
        | application/x-executable \
        | application/x-pie-executable \
        | application/vnd.microsoft.portable-executable)
            violations+=("$f ($mime)")
            ;;
    esac
done

if [ ${#violations[@]} -gt 0 ]; then
    echo "Refusing to commit compiled binaries:" >&2
    printf '  %s\n' "${violations[@]}" >&2
    echo >&2
    echo "Build outputs belong outside the repo. Use 'go build -o bin/...' or add the file to .gitignore." >&2
    exit 1
fi
