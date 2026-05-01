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

// Provide your own CSS file (overrides WithColor)
pico.New(pico.WithCustomCSS("myapp/overrides.css"))
```

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
