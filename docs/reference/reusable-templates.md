# Reusable Templates

Contrib apps provide named templates for common asset includes and UI components. Use these in your layout templates instead of hardcoding `staticURL` calls or repeating boilerplate HTML.

## Asset Includes

Include JavaScript assets in your layout `<head>`:

```html
<head>
    <link rel="stylesheet" href="{{ staticURL "app/app.min.css" }}">
    {{ template "htmx/js" . }}
    {{ template "htmx/config" . }}
</head>
```

The CSS bundle (Tailwind output) lives in your shell app under `internal/app/static/app.min.css`; see the [Tailwind guide](../guide/tailwind.md) for the recommended project layout.

### htmx

Provided by the [`htmx`](../contrib/htmx.md) contrib app.

| Template | Output |
|----------|--------|
| `{{ template "htmx/js" . }}` | `<script defer>` tag for htmx JS |
| `{{ template "htmx/config" . }}` | `<meta>` tag configuring htmx to swap `422` responses (for form validation) |
| `{{ template "htmx/dialog_script" . }}` | Listener that turns `htmx.OpenDialog`/`htmx.CloseDialog` events into native `dialog.showModal()` / `close()` calls |

## UI Components

### admin

Provided by the [`admin`](../contrib/admin.md) contrib app.

| Template | Description |
|----------|-------------|
| `{{ template "admin/pagination" (dict "BasePath" "/notes" "RawQuery" .RawQuery "Page" .Page) }}` | Offset-based pagination nav (Tailwind-classed) with query-preserving links and `aria-current="page"` on the active page |

## Icons

Each app that contributes nav items keeps its icon SVGs in `templates/icons.html` as `{{ define "<app>/icon_<name>" }}` blocks. Render an icon by template name with the core `{{ icon }}` function — see [Navigation › Icon convention](../guide/navigation.md#icon-convention).
