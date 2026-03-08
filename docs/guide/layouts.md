# Layouts & Rendering

The framework provides a layout system that wraps page content in a shared HTML shell. Layouts are CSS-agnostic — you bring your own CSS framework and templates.

## The Template System

Burrow uses Go's standard `html/template` package. Each app can contribute template files and template functions. At boot time, the framework:

1. Collects `.html` files from all `HasTemplates` apps
2. Collects static functions from all `HasFuncMap` apps
3. Parses everything into a single global `*template.Template`
4. Per request, clones the template and injects request-scoped functions from `HasRequestFuncMap` apps

Templates use `{{ define "appname/templatename" }}` blocks to namespace themselves:

```html
{{ define "notes/list" -}}
<h1>My Notes</h1>
<ul>
  {{ range .Notes }}
    <li>{{ .Title }}</li>
  {{ end }}
</ul>
{{- end }}
```

## Rendering in Handlers

Use `burrow.RenderTemplate()` to render a named template with data:

```go
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
    notes, err := h.repo.ListAll(r.Context())
    if err != nil {
        return err
    }

    return burrow.RenderTemplate(w, r, http.StatusOK, "notes/list", map[string]any{
        "Title": "My Notes",
        "Notes": notes,
    })
}
```

`RenderTemplate` does the following:

1. Executes the named template with the provided data, producing an HTML fragment
2. If the request has an `HX-Request: true` header (htmx), returns the fragment directly — no layout wrapping
3. Otherwise, passes the fragment to the layout function (if set) which wraps it in the full HTML shell
4. If no layout is set, returns the fragment as-is

This means the same handler automatically supports both full page loads and htmx partial updates.

## Layout Functions

A `LayoutFunc` receives the rendered page fragment and wraps it:

```go
type LayoutFunc func(w http.ResponseWriter, r *http.Request, code int, content template.HTML, data map[string]any) error
```

| Parameter | Description |
|-----------|-------------|
| `w` | HTTP response writer |
| `r` | HTTP request (for reading context values) |
| `code` | HTTP status code |
| `content` | The rendered template fragment as `template.HTML` |
| `data` | The same data map passed to `RenderTemplate` |

## Setting the App Layout

There are two ways to set the app layout:

**Using a design system app** (recommended):

```go
srv := burrow.NewServer(
    bootstrap.New(),  // provides base layout + CSS/JS assets
    // ... other apps
)
```

The `bootstrap` app injects its layout via middleware only when no layout is already set. This is batteries-included by default.

**Using `SetLayout()` explicitly:**

```go
srv.SetLayout(appLayout())
```

When `SetLayout()` is called, it takes precedence over design system middleware like `bootstrap`.

If neither approach is used, content renders unwrapped.

## Setting the Auth Layout

Public auth pages (login, register, recovery) typically shouldn't show the full app navbar. By default, `auth.New()` uses a built-in minimal auth layout (`DefaultAuthLayout()`) that renders a minimal HTML shell with Bootstrap CSS but no navigation. Authenticated auth routes (`/auth/credentials`, `/auth/recovery-codes`) continue to use the global app layout.

To override the auth layout with a custom one, use `auth.WithAuthLayout()`:

```go
auth.New(
    auth.WithAuthLayout(myCustomAuthLayout),
)
```

## Setting the Admin Layout

The admin layout is owned by the `admin` package. By default, `admin.New()` uses a built-in layout and dashboard renderer. To override with a custom layout:

```go
admin.New(admin.WithLayout(myCustomLayout))
```

Pass `nil` for no admin layout.

## Writing a Custom Layout

Layouts typically render another template from the global set. Here's how the Bootstrap app does it:

```go
func Layout() burrow.LayoutFunc {
    return func(w http.ResponseWriter, r *http.Request, code int, content template.HTML, data map[string]any) error {
        exec := burrow.TemplateExecutorFromContext(r.Context())

        layoutData := make(map[string]any, len(data)+2)
        maps.Copy(layoutData, data)
        layoutData["Content"] = content
        if _, ok := layoutData["Title"]; !ok {
            layoutData["Title"] = ""
        }

        html, err := exec(r, "bootstrap/layout", layoutData)
        if err != nil {
            return err
        }

        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.WriteHeader(code)
        _, err = w.Write([]byte(html))
        return err
    }
}
```

The corresponding template file:

```html
{{ define "bootstrap/layout" -}}
<!doctype html>
<html lang="{{ lang }}">
<head>
    <meta charset="utf-8">
    <title>{{ .Title }}</title>
    <link rel="stylesheet" href="{{ staticURL "bootstrap/bootstrap.min.css" }}">
</head>
<body>
    <nav>
        {{ range .NavItems }}
            <a href="{{ .URL }}">{{ .Icon }} {{ .Label }}</a>
        {{ end }}
    </nav>
    <main class="container py-4">
        {{ .Content }}
    </main>
    <script src="{{ staticURL "bootstrap/bootstrap.bundle.min.js" }}"></script>
</body>
</html>
{{- end }}
```

## Layout Unification

The app layout, auth layout, and admin layout all use the same context key (`burrow.Layout(ctx)`). The framework sets the app layout globally via middleware, while the auth and admin route groups override it with their own layouts. This means any handler can always rely on `burrow.Layout(ctx)` returning the correct layout for the current request.

## Available Context Values

These values are available in templates via FuncMap functions or in layout code via context helpers:

| Helper / FuncMap | Type | Set By |
|------------------|------|--------|
| `burrow.NavItems(ctx)` | `[]NavItem` | Framework (from all `HasNavItems` apps) |
| `burrow.Layout(ctx)` | `LayoutFunc` | Framework middleware / auth / admin |
| `{{ csrfToken }}` | `string` | CSRF app (`HasRequestFuncMap`) |
| `{{ lang }}` | `string` | i18n app (`HasRequestFuncMap`) |
| `{{ t "key" }}` | `string` | i18n app (`HasRequestFuncMap`) |
| `{{ staticURL "path" }}` | `string` | staticfiles app (`HasFuncMap`) |
| `{{ currentUser }}` | `*auth.User` | auth app (`HasRequestFuncMap`) |
| `{{ isAuthenticated }}` | `bool` | auth app (`HasRequestFuncMap`) |

See [Context Helpers](../reference/context-helpers.md) for the complete list.
