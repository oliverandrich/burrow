# Litewind

`contrib/litewind` is Burrow's Tailwind-utility design contrib. It ships a vendored static CSS file — no Tailwind CLI, no PostCSS, no node toolchain — and exposes the same `data-theme` switcher convention that `contrib/mucss` users will recognize.

## What it gives you

- A single `<link>` to a curated, static Tailwind-utility CSS — the full vocabulary you need for typical Burrow apps (typography, colors, spacing, flex/grid, borders, transitions, `dark:` variants)
- A drop-in theme switcher (auto / light / dark) backed by `localStorage` and `prefers-color-scheme`
- `iconSunFill`, `iconMoonStarsFill`, `iconCircleHalf` template functions auto-registered

What it does NOT give you (out of the box):

- Tailwind JIT — Litewind is static
- Arbitrary values (`bg-[#123456]`) — not supported by Litewind
- Container queries — not in the Litewind vocabulary

## Wiring

`contrib/litewind` ships no base layout — define your own and pass it via `srv.SetLayout(...)`. The css/theme-script/theme-switcher partials below are designed to be `{{ template … }}`'d from your layout.

```go
import (
    "github.com/oliverandrich/burrow"
    "github.com/oliverandrich/burrow/contrib/staticfiles"
    "github.com/oliverandrich/burrow/contrib/htmx"
    "github.com/oliverandrich/burrow/contrib/litewind"
)

func main() {
    srv := burrow.NewServer(
        staticfiles.New(),
        htmx.New(),
        litewind.New(),
        // your apps...
    )
    // ...
}
```

In your layout template:

```html
<head>
    {{ template "litewind/css" . }}
    {{ template "litewind/theme_script" . }}
</head>
<body>
    <nav>
        {{ template "litewind/theme_switcher" . }}
    </nav>
    {{ .Content }}
</body>
```

## Dark mode

`litewind/theme_script` sets `[data-theme="light"]` or `[data-theme="dark"]` on `<html>` from `localStorage.theme` (auto / light / dark). The vendored Litewind ships `dark:` variants targeting `[data-theme="dark"]`, so:

```html
<div class="bg-white text-gray-900 dark:bg-gray-900 dark:text-gray-100">
    ...
</div>
```

works out of the box. The inline script runs before first paint, so there is no FOUC when reloading a dark-mode page.

## Production optimization

With `Cache-Control: immutable` and Burrow's content-hashed static URLs the CSS loads once per release — fine for many apps.

> Note: this page describes the v0.20-pre `contrib/litewind` foundation. The v0.20 release is moving to Tailwind v4 via the standalone CLI (which already emits only the classes it sees, no separate purger needed). This page will be replaced by `tailwind.md` when the migration lands.
