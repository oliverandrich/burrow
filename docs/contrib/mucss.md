# µCSS

Design system using [µCSS v1.x](https://mucss.org/) and [htmx](https://htmx.org/). µCSS is built on PicoCSS v2 with the upstream Pico bugs fixed and a richer component set: Hero, Alert, Toast, Modal, Pagination, Badge, Skeleton, Spinner, Tabs, Accordion. Ships µCSS's default theme plus 20 named accent variants, a base layout, a navbar layout with overridable slots, and a dark/light/auto theme switcher.

**Package:** `github.com/oliverandrich/burrow/contrib/mucss`

**Depends on:** `staticfiles`, `htmx`

## Setup

```go
srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    auth.New(),
    mucss.New(),
    htmx.New(),
    healthcheck.New(),
    staticApp,
)
```

### Options

```go
// Pick a named accent color
mucss.New(mucss.WithColor(mucss.Blue))
mucss.New(mucss.WithColor(mucss.Zinc))
mucss.New(mucss.WithColor(mucss.Default))   // upstream default

// Flatter font scaling and tighter line-height for app-style UIs
mucss.New(mucss.WithCompactType())

// Combine
mucss.New(mucss.WithColor(mucss.Blue), mucss.WithCompactType())

// Provide your own CSS file (overrides WithColor and WithCompactType)
mucss.New(mucss.WithCustomCSS("myapp/overrides.css"))
```

## Layouts

### Base Layout (`mucss/layout`)

Minimal HTML shell with themed CSS, theme script, htmx, and the page content rendered directly in `<body>`. Apps control their own `<main>` / containers.

```go
mucss.Layout() // returns "mucss/layout"
```

This is the default layout injected by middleware when no other layout is set.

### Nav Layout (`mucss/nav_layout`)

Extends the base layout, wraps content in `<main class="container">`, and exposes overridable slots:

```go
srv.SetLayout(mucss.NavLayout()) // returns "mucss/nav_layout"
```

The nav layout renders, in order:

1. `{{ template "mucss/navbar" . }}` — empty by default, override in your app
2. `<main class="container">`
3. `{{ template "mucss/alerts" . }}` — empty by default, override for flash messages
4. `{{ .Content }}` — page content

To use it, define `mucss/navbar` and optionally `mucss/alerts` in your app's templates:

```html
{{ define "mucss/navbar" -}}
<nav class="container-fluid">
    <ul>
        <li><strong>My App</strong></li>
    </ul>
    <ul>
        <li>{{ template "mucss/theme_switcher" . }}</li>
    </ul>
</nav>
{{- end }}
```

## Color Variants

The default `mucss.New()` ships µCSS's default theme. You can pick from 20 named accent variants:

amber, azure, blue, cyan, fuchsia, green, grey, indigo, jade, lime, orange, pink, pumpkin, purple, red, sand, slate, violet, yellow, zinc.

```go
mucss.New(mucss.WithColor(mucss.Jade))
```

The full list is also available via `mucss.AllColors()`.

## Burrow extras (`mu-extras.min.css`)

A small `mu-extras.min.css` is always loaded alongside the main stylesheet (disabled when `WithCustomCSS` is set). It applies framework-level UX polish that isn't an upstream µCSS bug — currently:

- **Navbar dropdowns: minimum width** — `nav details.dropdown[open] > ul { min-width: 8rem }`. Prevents single-word menus (theme switcher, user menu) from looking orphaned.
- **Navbar SVG icons: vertical centering** — overrides `bsicons`' inline `vertical-align:-.125em` to `middle` inside `<nav>` so icon-and-text dropdown summaries align cleanly with sibling button heights.
- **Dialog header: flex layout for clean centering** — upstream µCSS uses `float:right` on `[rel="prev"]` / `.close`, which never aligns cleanly with the heading text beside it. The override switches `dialog > article > header` to `display:flex; align-items:center` and uses `order:1` + `margin-left:auto` on the close button so it stays at the right end regardless of DOM order while staying vertically centered with the title.

For right-anchored dropdowns (e.g. a theme switcher or user menu sitting on the right side of the navbar), use µCSS's intended pattern: `<ul dir="rtl">` to anchor the popup right, plus `<li dir="ltr">` on each item to keep the per-item text direction natural (icon-then-text, left-aligned). The `mucss/theme_switcher` template already does this.

To opt out of the extras entirely, use `WithCustomCSS` and ship your own complete stylesheet.

## Compact Typography (`WithCompactType`)

µCSS inherits Pico's blog-oriented defaults. On admin/app UIs at desktop widths, the default font-size grows up to ~21px and form padding feels generous. `WithCompactType()` ships an additional small stylesheet that tightens both.

**Globally:**

- `--mu-line-height: 1.4` (µCSS default `1.5`)

**On viewports ≥1024px** (mobile/tablet defaults are kept so touch targets stay within recommended sizes):

| Variable | µCSS default | Compact |
|---|---|---|
| `--mu-font-size` | up to 131.25% at 1536px | 106.25% (~17px) |
| `--mu-spacing` | 1rem | 0.85rem |
| `--mu-typography-spacing-vertical` | 1rem | 0.75rem |
| `--mu-form-element-spacing-vertical` | 0.75rem | 0.5rem |
| `--mu-form-element-spacing-horizontal` | 1rem | 0.75rem |
| `--mu-grid-column-gap` | 1rem | 0.75rem |
| `--mu-grid-row-gap` | 1rem | 0.75rem |

Navbar spacing (`--mu-nav-link-spacing-*`, `--mu-nav-element-spacing-*`) intentionally stays at the µCSS defaults — compact is meant to tighten content, not the navbar chrome.

µCSS's heading sizes and other rem-based metrics scale automatically with the base font-size, so they follow without further tweaks.

The override is loaded as a second `<link>` after the main µCSS stylesheet, so source-order cascade gives the override priority.

```go
mucss.New(mucss.WithCompactType())
```

`WithCompactType` is ignored when `WithCustomCSS` is set — custom CSS owns the entire stylesheet.

## Customization via CSS Variables

µCSS exposes ~200 CSS custom properties (`--mu-primary`, `--mu-spacing`, `--mu-border-radius`, `--mu-font-family`, …). To customize, ship your own CSS file and pass it via `WithCustomCSS`:

```css
/* myapp/static/myapp/overrides.css */
:root {
    --mu-primary: #6f42c1;
    --mu-border-radius: 0.5rem;
    --mu-spacing: 1.25rem;
}
```

```go
mucss.New(mucss.WithCustomCSS("myapp/overrides.css"))
```

`WithCustomCSS` replaces the bundled stylesheet entirely. Either include `@import` of one of the embedded `mucss/mu.{color}.css` files, or copy the upstream CSS into your override file as a base.

## Theme Switcher

Three-state switcher (light / dark / auto) using µCSS's `data-theme` attribute on `<html>`. Persisted in `localStorage["theme"]`.

```html
{{ template "mucss/theme_switcher" . }}
```

The switcher uses Unicode glyphs (☀ ☾ ◐) and is JS-free at the framework level (~30 lines of inline JS for the toggle behavior). `theme_script.html` is loaded inline in `<head>` and applies the saved theme before paint to avoid a flash of unstyled content.

The inner `<ul dir="rtl">` makes the dropdown right-anchored — Pico's default `left: 0` would overflow to the right when the switcher sits at the right end of the navbar.

## `<article>` is a card

µCSS (inheriting from Pico) styles `<article>` as a card: padding, rounded corners, a subtle multi-layer drop shadow (`--mu-card-box-shadow`), and a sectioning background for `<header>`/`<footer>`. Use `<article>` when you actually want a card; for thematic grouping without the card look, use `<section>` or `<div>`. The mucss.org demos themselves avoid `<article>` for non-card content for this reason.

If you nest `<article>` inside another card-like container (e.g. content inside `<dialog>`), the inner article keeps its own shadow — double shadows look heavy. Override with a scoped rule in your own CSS if needed:

```css
dialog article { box-shadow: none; background: transparent }
```

## Native `<dialog>` and Modal

µCSS styles native `<dialog>` elements via its modal component. No JavaScript framework needed — only the browser's `showModal()` / `close()` calls.

The `mucss/nav_layout` ships a permanent empty `<dialog id="modal"></dialog>` container plus the `htmx/dialog_script` listener, so HTMX-driven dialogs work out of the box. Each view renders its own `<article>` as the swapped content, so the view picks its width (e.g. `<article class="modal-lg">`) and any other classes. See [HTMX-driven dialogs](htmx.md#htmx-driven-dialogs) for the recommended pattern (open/close from the server via `htmx.OpenDialog` / `htmx.CloseDialog`).

## Pagination

`mucss/pagination` provides a `<nav class="pagination">` page navigator that integrates with the `paginate` package:

```html
{{ template "mucss/pagination" . }}
```

## Middleware Behaviour

The middleware injects `mucss/layout` **only when no layout is already set** in the request context:

- `srv.SetLayout(mucss.NavLayout())` is called — nav layout wins
- `srv.SetLayout()` is NOT called — base layout takes effect

## Updating µCSS

Run the just recipe to refresh the embedded CSS files:

```bash
just update-mucss            # latest pinned version
just update-mucss 1.4.7      # specific version
```

## Relationship to `contrib/bootstrap`

`contrib/mucss` is the default design contrib for Burrow. `contrib/bootstrap` is deprecated and scheduled for removal in v0.20 — only critical fixes are accepted in the meantime. New projects should use `mucss`; existing Bootstrap-based projects have one or two minor releases of runway to migrate.
