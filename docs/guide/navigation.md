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
            Icon:     "bi bi-journal-text",
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
| `Icon` | `string` | CSS icon class (e.g., Bootstrap Icons) |
| `Position` | `int` | Sort order (lower = earlier, stable sort preserves insertion order for equal positions) |
| `AuthOnly` | `bool` | Only show to authenticated users |
| `AdminOnly` | `bool` | Only show to admin users |

## Reading Nav Items in Layouts

The framework injects nav items into every request context via middleware. Read them with `burrow.NavItems(ctx)`:

```go
func appLayout(title string, content templ.Component) templ.Component {
    return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
        for _, item := range burrow.NavItems(ctx) {
            // Render navigation link...
        }
        // ...
        return nil
    })
}
```

## Filtering by Auth Status

The framework provides the raw nav items — filtering based on `AuthOnly` and `AdminOnly` is up to your layout. A typical pattern:

```go
user := auth.GetUser(r)
for _, item := range burrow.NavItems(ctx) {
    if item.AuthOnly && user == nil {
        continue
    }
    if item.AdminOnly && (user == nil || !user.IsAdmin()) {
        continue
    }
    // Render item...
}
```

## Ordering

Nav items are sorted by `Position` using a stable sort. Items with equal positions appear in app registration order. Suggested ranges:

| Range | Usage |
|-------|-------|
| 0–10 | Home, dashboard |
| 10–50 | Main app pages |
| 50–80 | Secondary pages |
| 80–100 | Admin, settings |
