# Layouts & Rendering

The framework provides a layout system that wraps page content in a shared HTML shell. Layouts are CSS-agnostic — you bring your own CSS framework and templates.

## Layout Functions

A `LayoutFunc` takes a page title and content component, returning a wrapped component:

```go
type LayoutFunc func(title string, content templ.Component) templ.Component
```

## Setting the App Layout

Set the app layout on the server before calling `Run()`:

```go
srv.SetLayout(appLayout)
```

If no layout is set, content renders unwrapped.

## Setting the Admin Layout

The admin layout is owned by the `admin` package. Pass it when creating the admin app:

```go
admin.New(adminLayout)
```

Pass `nil` for no admin layout.

## Writing a Layout

Layouts read framework values from the request context:

```go
func appLayout(title string, content templ.Component) templ.Component {
    return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
        // Read framework context values.
        navItems := burrow.NavItems(ctx)
        csrfToken := csrf.Token(ctx)

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
            for _, item := range burrow.NavItems(ctx) {
                <a href={ templ.SafeURL(item.URL) }>{ item.Label }</a>
            }
        </nav>
        @content
    </body>
    </html>
}
```

## Rendering in Handlers

Use `burrow.Render()` to render a Templ component with a status code:

```go
func (h *Handlers) HomePage(w http.ResponseWriter, r *http.Request) error {
    return burrow.Render(w, r, http.StatusOK, homePageComponent())
}
```

`burrow.Render()` calls `component.Render()` with the request context, so all context values (nav items, CSRF token, locale, current user) are available to the template.

## Available Context Values

| Helper | Type | Set By |
|--------|------|--------|
| `burrow.NavItems(ctx)` | `[]NavItem` | Framework (from all `HasNavItems` apps) |
| `burrow.Layout(ctx)` | `LayoutFunc` | Framework middleware |
| `admin.Layout(ctx)` | `LayoutFunc` | Admin middleware |
| `csrf.Token(ctx)` | `string` | CSRF middleware |
| `i18n.Locale(ctx)` | `string` | i18n middleware |
| `i18n.T(ctx, key)` | `string` | i18n middleware |
| `staticfiles.URL(ctx, name)` | `string` | staticfiles middleware |

See [Context Helpers](../reference/context-helpers.md) for the complete list.
