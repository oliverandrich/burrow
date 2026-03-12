# Bootstrap

Swappable design system using [Bootstrap 5](https://getbootstrap.com/) and [htmx](https://htmx.org/). Provides static assets, a base HTML layout, and a dark mode theme switcher for all pages. For icons, see [`bsicons`](bsicons.md). For htmx helpers, see [`htmx`](htmx.md).

**Package:** `github.com/oliverandrich/burrow/contrib/bootstrap`

**Depends on:** `staticfiles`, `htmx`

## Setup

```go
srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    auth.New(),
    bootstrap.New(),                    // provides base layout + Bootstrap assets
    htmx.New(),                         // serves htmx.min.js
    healthcheck.New(),
    admin.New(),
    staticApp, // staticfiles.New(myStaticFS) — returns (*App, error)
)
```

The `staticfiles` app must be registered to serve the embedded CSS and JS.

## Layout

`Layout()` returns the layout template name (`"bootstrap/layout"`) for a base HTML page with Bootstrap CSS, Bootstrap JS bundle (includes Popper), and htmx:

```go
bootstrap.Layout() // returns "bootstrap/layout"
```

The layout template:

1. Receives the rendered page fragment as `.Content` (injected automatically by `RenderTemplate`)
2. Includes CSRF meta tag and theme switcher
3. Accesses dynamic data (navigation via `navLinks`, current user, etc.) via template functions from the framework and `HasRequestFuncMap` apps

## Middleware Behaviour

The bootstrap middleware injects its layout **only when no layout is already set** in the request context:

- `srv.SetLayout("custom/layout")` is called — custom layout wins, bootstrap skips
- `srv.SetLayout()` is NOT called — bootstrap layout (`"bootstrap/layout"`) takes effect
- Admin `/admin` route group always overrides unconditionally

This makes bootstrap batteries-included by default without fighting custom layouts.

## Templates

The bootstrap app implements `HasTemplates` and contributes these templates:

| Template | Description |
|----------|-------------|
| `bootstrap/layout` | Base HTML page shell with theme switcher, CSS/JS |
| `bootstrap/pagination` | Offset-based pagination nav component |
| `bootstrap/theme_script` | Inline script for dark mode persistence |
| `bootstrap/theme_switcher` | Theme toggle button component |

### Pagination

Use the pagination template in your own templates:

```html
{{ define "notes/list" -}}
<h1>Notes</h1>
<ul>
  {{ range .Notes }}<li>{{ .Title }}</li>{{ end }}
</ul>
{{ template "bootstrap/pagination" .Page }}
{{- end }}
```

The pagination template expects a `burrow.PageResult` value. It renders nothing when `TotalPages <= 1`, shows previous/next buttons with disabled states, and includes ellipsis for large page counts.

For full pagination documentation, see the [Pagination Guide](../guide/pagination.md).

## Static Files

The bootstrap app embeds these static assets and implements `HasStaticFiles` to contribute them under the `"bootstrap"` prefix:

| File | Description |
|------|-------------|
| `bootstrap.min.css` | Bootstrap 5 CSS |
| `bootstrap.bundle.min.js` | Bootstrap 5 JS bundle (includes Popper) |

These are served at `/static/bootstrap/bootstrap.min.css`, etc. when the `staticfiles` app is registered.

!!! note "htmx is a separate app"
    The htmx JavaScript file is served by the dedicated [`htmx` contrib app](htmx.md), not by the bootstrap app.

## Template Functions

The bootstrap app contributes these template functions via `HasFuncMap`:

| Function | Example | Description |
|----------|---------|-------------|
| `add` | `{{ add .Page 1 }}` | Integer addition |
| `sub` | `{{ sub .Total 1 }}` | Integer subtraction |
| `pageURL` | `{{ pageURL .BaseURL .Page .Limit }}` | Builds a pagination URL |
| `pageLimit` | `{{ pageLimit .Limit }}` | Returns the current page size |
| `pageNumbers` | `{{ range pageNumbers .Current .Total }}` | Generates page number slice for pagination controls |

These are used internally by the `bootstrap/pagination` template but are available in all templates.

## Dark Mode

The layout includes a theme switcher toggle that persists the user's preference in `localStorage`. It uses Bootstrap's `data-bs-theme` attribute to switch between light and dark modes without a page reload.

## Swapping Design Systems

The `bootstrap` package is intentionally self-contained. To use a different CSS framework, create a new contrib app (e.g., `contrib/pico` or `contrib/tailwind`) that provides its own assets and layout, and register it instead of `bootstrap.New()`.

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `HasStaticFiles` | Contributes embedded Bootstrap assets under `"bootstrap"` prefix |
| `HasMiddleware` | Injects bootstrap layout when no layout is set in context |
| `HasTemplates` | Contributes layout, pagination, and alert templates |
| `HasFuncMap` | Contributes icon and utility template functions |
| `HasDependencies` | Requires `staticfiles` and `htmx` |
