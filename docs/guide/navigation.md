# Navigation

Apps contribute navigation items that are collected and made available to layouts via context.

## Contributing Nav Items

Implement the `HasNavItems` interface:

```go
// notesIcon is an inline SVG (Bootstrap Icons' "journal-text"); copy any
// icon SVG into a template.HTML const next to your app.
const notesIcon = template.HTML(`<svg xmlns="http://www.w3.org/2000/svg" width="1em" height="1em" fill="currentColor" viewBox="0 0 16 16">…</svg>`)

func (a *App) NavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {
            Label:    "Notes",
            URL:      "/notes",
            Icon:     notesIcon,
            Position: 20,
            AuthOnly: true,
        },
    }
}
```

## NavItem Fields

| Field | Type | Description |
|-------|------|-------------|
| `Label` | `string` | Display text for the link |
| `LabelKey` | `string` | i18n message ID; translated at render time, falls back to `Label` |
| `URL` | `string` | Target path |
| `Icon` | `template.HTML` | Inline SVG icon (copy from any icon set as `template.HTML`); empty string for no icon |
| `Position` | `int` | Sort order (lower = earlier, stable sort preserves insertion order for equal positions) |
| `AuthOnly` | `bool` | Only show to authenticated users |
| `AdminOnly` | `bool` | Only show to admin users |

## Using Nav Items in Templates

The framework provides the `navLinks` template function, which returns filtered, template-ready `NavLink` values. It automatically:

1. Filters out `AuthOnly` items when no user is authenticated
2. Filters out `AdminOnly` items when the user is not an admin
3. Computes `IsActive` based on the current request path

```html
{{ define "app/layout" -}}
<nav>
  {{ range navLinks }}
    <a href="{{ .URL }}"{{ if .IsActive }} class="active"{{ end }}>
      {{ .Icon }} {{ .Label }}
    </a>
  {{ end }}
</nav>
<main>{{ .Content }}</main>
{{- end }}
```

### NavLink Fields

| Field | Type | Description |
|-------|------|-------------|
| `Label` | `string` | Display text |
| `URL` | `string` | Target path |
| `Icon` | `template.HTML` | Inline SVG icon |
| `IsActive` | `bool` | `true` when the current request path matches the item URL |

Active matching uses prefix matching: `/notes/1` matches `/notes`. The root URL `/` only matches exactly.

### Raw Nav Items

The `navItems` template function is still available and returns the raw `[]NavItem` values without filtering or active-state computation. Use it when you need full control over rendering logic.

## Using Nav Items in Go Code

Read nav items with `burrow.NavItems(ctx)`:

```go
navItems := burrow.NavItems(r.Context())
```

## Auth Filtering

The `navLinks` function reads an `AuthChecker` from the request context to determine visibility. The `auth` contrib app injects this automatically via its middleware — no manual wiring needed. When no `AuthChecker` is present (e.g., no auth app installed), `AuthOnly` and `AdminOnly` items are hidden by default.

If you use a custom auth system instead of the `auth` contrib app, inject an `AuthChecker` in your middleware:

```go
ctx = burrow.WithAuthChecker(ctx, burrow.AuthChecker{
    IsAuthenticated: func() bool { return user != nil },
    IsAdmin:         func() bool { return user != nil && user.IsAdmin },
})
```

## Ordering

Nav items are sorted by `Position` using a stable sort. Items with equal positions appear in app registration order. Suggested ranges:

| Range | Usage |
|-------|-------|
| 0–10 | Home, dashboard |
| 10–50 | Main app pages |
| 50–80 | Secondary pages |
| 80–100 | Admin, settings |
