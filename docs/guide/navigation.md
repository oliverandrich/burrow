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
            Icon:     "notes/icon_journal_text",
            Position: 20,
            AuthOnly: true,
        },
    }
}
```

`Icon` is the name of a template define. See [Icon convention](#icon-convention) below.

## NavItem Fields

| Field | Type | Description |
|-------|------|-------------|
| `Label` | `string` | Display text for the link; also doubles as the i18n message ID (see [Translation](#translation)) |
| `URL` | `string` | Target path |
| `Icon` | `string` | Name of a template define (e.g. `"notes/icon_journal_text"`); empty string for no icon |
| `Position` | `int` | Sort order (lower = earlier, stable sort preserves insertion order for equal positions) |
| `AuthOnly` | `bool` | Only show to authenticated users |
| `AdminOnly` | `bool` | Only show to admin users |

### Translation

`navLinks` pipes `Label` through `i18n.T` at render time, so each Label is also the i18n message ID for its translations. Contribute translations keyed by the English Label:

```toml
# active.de.toml
Notes = "Notizen"
Settings = "Einstellungen"
```

If no translation matches the Label, the raw Label is rendered — so an app without translations Just Works in English. See [i18n: Labels as Keys](i18n.md#labels-as-keys) for the trade-off (silently invalidating translations when a Label is reworded) and when a structured i18n key is the better choice.

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
      {{ if .Icon }}{{ icon .Icon }} {{ end }}{{ .Label }}
    </a>
  {{ end }}
</nav>
<main>{{ .Content }}</main>
{{- end }}
```

The built-in `{{ icon }}` template function looks up the named template and renders it. (Go's built-in `{{ template "name" }}` action requires a string literal at parse time and can't accept a variable.)

### NavLink Fields

| Field | Type | Description |
|-------|------|-------------|
| `Label` | `string` | Display text |
| `URL` | `string` | Target path |
| `Icon` | `string` | Template-define name (see [Icon convention](#icon-convention)) |
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

## Icon convention

`NavItem.Icon` is the name of a template define. Each app keeps its icons in a `templates/icons.html` file, namespaced like the rest of the app's templates:

```html
{{ define "notes/icon_journal_text" -}}<svg xmlns="http://www.w3.org/2000/svg" width="1em" height="1em" fill="currentColor" viewBox="0 0 16 16"><path d="…"/></svg>{{- end }}
```

Layouts render the icon via the `{{ icon }}` template function: `{{ icon .Icon }}`. The same defines are also callable from the app's own templates via the built-in `{{ template "notes/icon_journal_text" . }}` action — so each icon lives in exactly one place. Copy SVGs from any icon set (e.g. [Bootstrap Icons](https://icons.getbootstrap.com/)) and size them with Tailwind utilities applied to the `<svg>` element directly.
