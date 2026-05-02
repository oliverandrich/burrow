# Pico

Design system using [PicoCSS v2.x](https://picocss.com/) and [htmx](https://htmx.org/). Ships PicoCSS's default theme plus 19 named accent variants, a base layout, a navbar layout with overridable slots, and a dark/light/auto theme switcher. PicoCSS is class-light and semantic — most styling applies to native HTML elements without classes.

**Package:** `github.com/oliverandrich/burrow/contrib/pico`

**Depends on:** `staticfiles`, `htmx`

## Setup

```go
srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    auth.New(),
    pico.New(),                    // default theme, htmx enabled
    htmx.New(),
    healthcheck.New(),
    staticApp,
)
```

### Options

```go
// Pick a named accent color
pico.New(pico.WithColor(pico.Blue))
pico.New(pico.WithColor(pico.Zinc))
pico.New(pico.WithColor(pico.Default))   // upstream default

// Flatter font scaling and tighter line-height for app-style UIs
pico.New(pico.WithCompactType())

// Combine
pico.New(pico.WithColor(pico.Blue), pico.WithCompactType())

// Provide your own CSS file (overrides WithColor, WithCompactType,
// and the upstream-Pico fixes — your CSS owns everything)
pico.New(pico.WithCustomCSS("myapp/overrides.css"))
```

### Fixes for upstream Pico bugs

A small `pico-fixes.min.css` is always loaded alongside the main stylesheet (unless `WithCustomCSS` is set). The same content is shared as a [public gist](https://gist.github.com/oliverandrich/de1c83ca7874e8162b07947e1b768b88) for Pico users outside Burrow. It currently contains:

- **Firefox dropdown positioning** ([picocss/pico#701](https://github.com/picocss/pico/issues/701)) — `nav details.dropdown { display: inline-block }`. Pico defaults to `display: inline`, which breaks `<details>`-based dropdowns on Firefox: inline elements do not reliably establish a positioning context for the absolutely-positioned submenu, so the menu lands at the page's top-left instead of below the toggle. `inline-block` gives the same inline flow with a proper positioning context.
- **Tooltip positioning near viewport edges** ([picocss/pico#694](https://github.com/picocss/pico/pull/694)) — `[data-tooltip] { display: inline-block }`. Same root cause: tooltips fail to position correctly near edges because the inline parent does not establish a positioning context for the absolutely-positioned tooltip pseudo-element.
- **`<small>` font-size never applied** ([picocss/pico#561](https://github.com/picocss/pico/pull/561)) — `small { font-size: var(--pico-font-size) }`. Pico sets `--pico-font-size: 0.875em` on `<small>` but does not apply it as the actual `font-size`, so the user-agent's `smaller` keyword wins. Applying the var fixes the size to Pico's intended value.

## Layouts

### Base Layout (`pico/layout`)

Minimal HTML shell with themed CSS, theme script, htmx, and the page content rendered directly in `<body>`. Apps control their own `<main>` / containers.

```go
pico.Layout() // returns "pico/layout"
```

This is the default layout injected by middleware when no other layout is set.

### Nav Layout (`pico/nav_layout`)

Extends the base layout, wraps content in `<main class="container">`, and exposes overridable slots:

```go
srv.SetLayout(pico.NavLayout()) // returns "pico/nav_layout"
```

The nav layout renders, in order:

1. `{{ template "pico/navbar" . }}` — empty by default, override in your app
2. `<main class="container">`
3. `{{ template "pico/alerts" . }}` — empty by default, override for flash messages
4. `{{ .Content }}` — page content

To use it, define `pico/navbar` and optionally `pico/alerts` in your app's templates:

```html
{{ define "pico/navbar" -}}
<nav class="container-fluid">
    <ul>
        <li><strong>My App</strong></li>
    </ul>
    <ul>
        <li><a href="/">Home</a></li>
    </ul>
</nav>
{{- end }}
```

## Color Variants

The default `pico.New()` ships PicoCSS's default theme. You can pick from 19 named accent variants:

amber, blue, cyan, fuchsia, green, grey, indigo, jade, lime, orange, pink, pumpkin, purple, red, sand, slate, violet, yellow, zinc.

```go
pico.New(pico.WithColor(pico.Jade))
```

The full list is also available via `pico.AllColors()`.

## Compact Typography (`WithCompactType`)

PicoCSS's defaults are tuned for long-form blog content. On admin/app UIs at desktop widths, the default font-size grows up to ~21px and form padding feels generous. `WithCompactType()` ships an additional small stylesheet that tightens both.

**Globally:**

- `--pico-line-height: 1.4` (Pico default `1.5`)

**On viewports ≥1024px** (mobile/tablet defaults are kept so touch targets stay within recommended sizes):

| Variable | Pico default | Compact |
|---|---|---|
| `--pico-font-size` | up to 131.25% at 1536px | 106.25% (~17px) |
| `--pico-spacing` | 1rem | 0.85rem |
| `--pico-typography-spacing-vertical` | 1rem | 0.75rem |
| `--pico-form-element-spacing-vertical` | 0.75rem | 0.5rem |
| `--pico-form-element-spacing-horizontal` | 1rem | 0.75rem |
| `--pico-nav-link-spacing-vertical` | 1rem | 0.5rem |
| `--pico-nav-element-spacing-vertical` | 1rem | 0.5rem |
| `--pico-grid-column-gap` | 1rem | 0.75rem |
| `--pico-grid-row-gap` | 1rem | 0.75rem |

Pico's heading sizes and other rem-based metrics scale automatically with the base font-size, so they follow without further tweaks.

The override is loaded as a second `<link>` after the main pico stylesheet, so source-order cascade gives the override priority over Pico's media queries at 1280px and 1536px.

```go
pico.New(pico.WithCompactType())
```

`WithCompactType` is ignored when `WithCustomCSS` is set — custom CSS owns the entire stylesheet.

## Customization via CSS Variables

PicoCSS exposes ~150 CSS custom properties (`--pico-primary`, `--pico-spacing`, `--pico-border-radius`, `--pico-font-family`, …). To customize, ship your own CSS file and pass it via `WithCustomCSS`:

```css
/* myapp/static/myapp/overrides.css */
:root {
    --pico-primary: #6f42c1;
    --pico-border-radius: 0.5rem;
    --pico-spacing: 1.25rem;
}
```

```go
pico.New(pico.WithCustomCSS("myapp/overrides.css"))
```

`WithCustomCSS` replaces the bundled stylesheet entirely — your file is the only stylesheet loaded. Either include `@import` of one of the embedded `pico/*.min.css` files, or copy the upstream CSS into your override file as a base.

## Theme Switcher

Three-state switcher (light / dark / auto) using PicoCSS's `data-theme` attribute on `<html>`. Persisted in `localStorage["theme"]` (same key as `contrib/bootstrap`, so the user-side preference survives a Bootstrap → Pico migration).

```html
{{ template "pico/theme_switcher" . }}
```

The switcher uses Unicode glyphs (☀ ☾ ◐) instead of an icon library to avoid pulling in a dependency. `theme_script.html` is loaded inline in `<head>` and applies the saved theme before paint to avoid a flash of unstyled content.

## Native `<dialog>`

PicoCSS styles native `<dialog>` elements out of the box. No JavaScript framework needed — only the browser's `showModal()` / `close()` calls. See `example/hello-pico` for a working example.

## Pagination

`pico/pagination` provides a `<nav>` + `<ul>` page navigator that integrates with the `paginate` package:

```html
{{ template "pico/pagination" . }}
```

## Middleware Behaviour

The middleware injects `pico/layout` **only when no layout is already set** in the request context:

- `srv.SetLayout(pico.NavLayout())` is called — nav layout wins
- `srv.SetLayout()` is NOT called — base layout takes effect

## Updating PicoCSS

Run the just recipe to refresh the embedded CSS files:

```bash
just update-pico            # latest pinned version
just update-pico 2.1.1      # specific version
```

## Relationship to `contrib/bootstrap`

`contrib/pico` is an **alternative** to `contrib/bootstrap`, not a replacement. Both can be registered, but they share template names for error pages (`error/401`, `error/default`, …), so the last-registered app wins for those names. Pick one design contrib per server.
