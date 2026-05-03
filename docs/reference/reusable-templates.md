# Reusable Templates

Contrib apps provide named templates for common asset includes and UI components. Use these in your layout templates instead of hardcoding `staticURL` calls or repeating boilerplate HTML.

## Asset Includes

Include CSS and JavaScript assets in your layout `<head>`:

```html
<head>
    {{ template "mucss/css" . }}
    {{ template "mucss/theme_script" . }}
    {{ template "htmx/js" . }}
    {{ template "htmx/config" . }}
</head>
```

### mucss

Provided by the [`mucss`](../contrib/mucss.md) contrib app — the default design contrib.

| Template | Output |
|----------|--------|
| `{{ template "mucss/css" . }}` | `<link>` tags for the µCSS stylesheet (and `mu-extras.min.css` unless `WithCustomCSS` is set) |
| `{{ template "mucss/theme_script" . }}` | Inline script that applies the persisted theme before paint (place in `<head>`) |

### htmx

Provided by the [`htmx`](../contrib/htmx.md) contrib app.

| Template | Output |
|----------|--------|
| `{{ template "htmx/js" . }}` | `<script defer>` tag for htmx JS |
| `{{ template "htmx/config" . }}` | `<meta>` tag configuring htmx to swap `422` responses (for form validation) |
| `{{ template "htmx/dialog_script" . }}` | Listener that turns `htmx.OpenDialog`/`htmx.CloseDialog` events into native `dialog.showModal()` / `close()` calls |

### alpine

Provided by the [`alpine`](../contrib/alpine.md) contrib app.

| Template | Output |
|----------|--------|
| `{{ template "alpine/js" . }}` | `<script defer>` tag for Alpine.js |

### bootstrap (deprecated, removed in v0.20)

Provided by the deprecated [`bootstrap`](../contrib/bootstrap.md) contrib app. New projects should use `mucss` above.

| Template | Output |
|----------|--------|
| `{{ template "bootstrap/css" . }}` | `<link>` tag for Bootstrap CSS |
| `{{ template "bootstrap/js" . }}` | `<script defer>` tag for Bootstrap JS bundle (includes Popper) |

## UI Components

### mucss

Provided by the [`mucss`](../contrib/mucss.md) contrib app.

| Template | Description |
|----------|-------------|
| `{{ template "mucss/layout" . }}` | Base HTML page shell with theme support |
| `{{ template "mucss/nav_layout" . }}` | Layout with navbar and alerts slots; ships a permanent `<dialog id="modal">` and the `htmx/dialog_script` |
| `{{ template "mucss/pagination" (dict "BasePath" "/notes" "RawQuery" .RawQuery "Page" .Page) }}` | Offset-based pagination nav with query-preserving links and `aria-current="page"` on the active page |
| `{{ template "mucss/theme_switcher" . }}` | Three-state theme toggle (light/dark/auto), `aria-current="true"` marks the active option |

### bootstrap (deprecated, removed in v0.20)

| Template | Description |
|----------|-------------|
| `{{ template "bootstrap/layout" . }}` | Base HTML page shell with theme support |
| `{{ template "bootstrap/pagination" dict "BasePath" "/notes" "RawQuery" .RawQuery "Page" .Page }}` | Offset-based pagination nav with query-preserving links |
| `{{ template "bootstrap/theme_script" . }}` | Inline script for dark mode persistence (place in `<head>`) |
| `{{ template "bootstrap/theme_switcher" . }}` | Theme toggle button (light/dark/auto) |
