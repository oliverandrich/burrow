# Reusable Templates

Contrib apps provide named templates for common asset includes and UI components. Use these in your layout templates instead of hardcoding `staticURL` calls or repeating boilerplate HTML.

## Asset Includes

Include CSS and JavaScript assets in your layout `<head>`:

```html
<head>
    {{ template "bootstrap/css" . }}
    {{ template "bootstrap/js" . }}
    {{ template "htmx/js" . }}
    {{ template "htmx/config" . }}
</head>
```

### bootstrap

Provided by the [`bootstrap`](../contrib/bootstrap.md) contrib app.

| Template | Output |
|----------|--------|
| `{{ template "bootstrap/css" . }}` | `<link>` tag for Bootstrap CSS |
| `{{ template "bootstrap/js" . }}` | `<script defer>` tag for Bootstrap JS bundle (includes Popper) |

### htmx

Provided by the [`htmx`](../contrib/htmx.md) contrib app.

| Template | Output |
|----------|--------|
| `{{ template "htmx/js" . }}` | `<script defer>` tag for htmx JS |
| `{{ template "htmx/config" . }}` | `<meta>` tag configuring htmx to swap `422` responses (for form validation) |

## UI Components

### bootstrap

Provided by the [`bootstrap`](../contrib/bootstrap.md) contrib app.

| Template | Description |
|----------|-------------|
| `{{ template "bootstrap/layout" . }}` | Base HTML page shell with theme support |
| `{{ template "bootstrap/pagination" .Page }}` | Offset-based pagination nav (expects `burrow.PageResult`) |
| `{{ template "bootstrap/theme_script" . }}` | Inline script for dark mode persistence (place in `<head>`) |
| `{{ template "bootstrap/theme_switcher" . }}` | Theme toggle button (light/dark/auto) |
