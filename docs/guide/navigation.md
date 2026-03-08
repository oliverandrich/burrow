# Navigation

Apps contribute navigation items that are collected and made available to layouts via context.

## Contributing Nav Items

Implement the `HasNavItems` interface:

```go
func (a *App) NavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {
            Label:    "Notes",
            URL:      "/notes",
            Icon:     bsicons.JournalText(),
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
| `Icon` | `template.HTML` | Inline SVG icon (e.g., `bsicons.House()`), empty string for no icon |
| `Position` | `int` | Sort order (lower = earlier, stable sort preserves insertion order for equal positions) |
| `AuthOnly` | `bool` | Only show to authenticated users |
| `AdminOnly` | `bool` | Only show to admin users |

## Reading Nav Items in Templates

The framework injects nav items into every request context via middleware. In layout templates, they are typically passed as `.NavItems`:

```html
{{ define "app/layout" -}}
<nav>
  {{ range .NavItems }}
    <a href="{{ .URL }}">{{ .Icon }} {{ .Label }}</a>
  {{ end }}
</nav>
<main>{{ .Content }}</main>
{{- end }}
```

In Go code, read them with `burrow.NavItems(ctx)`:

```go
navItems := burrow.NavItems(r.Context())
```

## Filtering by Auth Status

The framework provides the raw nav items — filtering based on `AuthOnly` and `AdminOnly` is up to your layout. A typical pattern:

```go
user := auth.UserFromContext(r.Context())
for _, item := range burrow.NavItems(r.Context()) {
    if item.AuthOnly && user == nil {
        continue
    }
    if item.AdminOnly && (user == nil || !user.IsAdmin()) {
        continue
    }
    // Render item...
}
```

In templates, the [Bootstrap layout](../contrib/bootstrap.md#layout) handles this automatically.

## Ordering

Nav items are sorted by `Position` using a stable sort. Items with equal positions appear in app registration order. Suggested ranges:

| Range | Usage |
|-------|-------|
| 0–10 | Home, dashboard |
| 10–50 | Main app pages |
| 50–80 | Secondary pages |
| 80–100 | Admin, settings |
