# Layouts & Rendering

The framework provides a layout system that wraps page content in a shared HTML shell. Layouts are CSS-agnostic — you bring your own CSS framework and templates.

## Layout Functions

A `LayoutFunc` takes a page title and content component, returning a wrapped component:

```go
type LayoutFunc func(title string, content templ.Component) templ.Component
```

## Setting Layouts

The `Layouts` struct holds two optional layout slots:

```go
srv.SetLayouts(core.Layouts{
    App:   appLayout,   // User-facing pages (login, dashboard, etc.)
    Admin: adminLayout, // Admin pages (user management, etc.)
})
```

Both fields are optional. If a layout is `nil`, content renders unwrapped.

## Writing a Layout

Layouts read framework values from the request context:

```go
func appLayout(title string, content templ.Component) templ.Component {
    return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
        // Read framework context values.
        navItems := core.NavItems(ctx)
        csrfToken := core.CSRFToken(ctx)

        _, _ = io.WriteString(w, "<!DOCTYPE html><html><head><title>")
        _, _ = io.WriteString(w, title)
        _, _ = io.WriteString(w, "</title></head><body>")

        // Render navigation.
        for _, item := range navItems {
            _, _ = io.WriteString(w, `<a href="`)
            _, _ = io.WriteString(w, item.URL)
            _, _ = io.WriteString(w, `">`)
            _, _ = io.WriteString(w, item.Label)
            _, _ = io.WriteString(w, `</a> `)
        }

        // Render page content.
        if err := content.Render(ctx, w); err != nil {
            return err
        }

        _, _ = io.WriteString(w, "</body></html>")
        return nil
    })
}
```

With Templ templates, the layout is cleaner:

```
// templates/layouts/app.templ
templ AppLayout(title string, content templ.Component) {
    <!DOCTYPE html>
    <html>
    <head><title>{ title }</title></head>
    <body>
        <nav>
            for _, item := range core.NavItems(ctx) {
                <a href={ templ.SafeURL(item.URL) }>{ item.Label }</a>
            }
        </nav>
        @content
    </body>
    </html>
}
```

## Rendering in Handlers

Use `core.Render()` to render a Templ component with a status code:

```go
func (h *Handlers) HomePage(c *echo.Context) error {
    return core.Render(c, http.StatusOK, homePageComponent())
}
```

`core.Render()` calls `component.Render()` with the request context, so all context values (nav items, CSRF token, locale, current user) are available to the template.

## Available Context Values

| Helper | Type | Set By |
|--------|------|--------|
| `core.NavItems(ctx)` | `[]NavItem` | Framework (from all `HasNavItems` apps) |
| `core.CSRFToken(ctx)` | `string` | CSRF middleware |
| `i18n.Locale(ctx)` | `string` | i18n middleware |
| `i18n.T(ctx, key)` | `string` | i18n middleware |
| `staticfiles.URL(ctx, name)` | `string` | staticfiles middleware |

See [Context Helpers](../reference/context-helpers.md) for the complete list.
