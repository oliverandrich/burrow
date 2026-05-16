# burrow-purge

`cmd/burrow-purge` is an optional Go-native CSS purger for Burrow apps using `contrib/litewind`. It strips unused rules from the vendored Litewind CSS down to only what your templates reference — typical output is in the 5-20KB-gzip range.

No node, no PostCSS, no purgecss-binary. Just Go.

## Install

In your app's `go.mod`:

```bash
go get -tool github.com/oliverandrich/burrow/cmd/burrow-purge
```

## Usage

In the simplest case — your app's templates live under `templates/` and you want one CSS file shipped at `static/app.min.css`:

```bash
go tool burrow-purge --out static/app.min.css
```

That command:

- Loads `contrib/litewind/static/litewind.min.css` from the Burrow module (resolved via `go list -m`)
- Scans `templates/**/*.html` plus every `**/*.html` under Burrow's `contrib/` (so contrib UIs you mount get their classes in too)
- Writes the filtered, minified CSS to `static/app.min.css`

## Flags

| Flag | Default | Notes |
|---|---|---|
| `--out <path>` | (required) | Output CSS file |
| `--css <path>` | Burrow's vendored Litewind | Source CSS to purge |
| `--content <glob>` | `templates/**/*.html` | Content glob; repeatable |
| `--no-burrow-contribs` | off | Skip auto-include of Burrow contrib templates |
| `--keep <class>` | (none) | Force-keep a class not detected in content; repeatable |
| `--verbose` | off | Per-file extraction log to stderr |

## What gets kept

- Selectors with no class component (`body`, `:root`, `*`, `::before`, ...) — always kept
- Selectors with classes — kept only when every class in the selector appears in the scanned content
- At-rules: `@font-face`, `@keyframes`, `@page`, `@property`, `@counter-style` — kept verbatim (their inner selectors are not utility classes)
- At-rules: `@media`, `@supports`, `@container`, `@layer`, `@scope` — recursively filtered; wrapper dropped when no inner rule survives
- `@charset`, `@import` and other single-line at-rules — kept verbatim

## What burrow-purge does NOT do

- **No JIT.** Designed for Litewind's static utility set. Tailwind arbitrary values like `bg-[#123456]` would need a full Tailwind toolchain.
- **No dynamic class detection.** `class="text-{{ color }}"` resolves nothing at scan time — write `class="text-red-500 text-green-500 text-blue-500 ..."` and let purge keep the ones you use, or use `--keep` for known runtime injections.
- **No plugin system.** It's ~600 lines of Go that solve one problem well.

## Edge cases

- **Whitespace inside `class="..."` is preserved**: `<div class="btn {% if x %}btn-primary{% endif %}">` extracts `btn`, `btn-primary`, plus harmless `if`/`x`/`endif` tokens that match no CSS rule.
- **CSS escapes round-trip**: Litewind's `.hover\:bg-red-500` selector and the markup-side `class="hover:bg-red-500"` resolve to the same class identifier.
- **Compound selectors require all classes**: `.flex.items-center { ... }` is dropped unless *both* `flex` and `items-center` appear in scanned content.
- **Multi-selector lists are split**: `.kept, .dropped { color: red }` becomes `.kept { color: red }` when only the first matches.

## How burrow-purge resolves Burrow's contrib templates

The tool runs `go list -m -f '{{.Dir}}' github.com/oliverandrich/burrow` to find the directory of the burrow module that your project depends on. That directory contains `contrib/<app>/templates/*.html` for every contrib app. The lookup honors `replace` directives, vendor mode, and the module cache.

If you do not want Burrow's contrib templates included (e.g. you do not mount admin/auth/jobs), pass `--no-burrow-contribs` and only your own globs will be scanned.
