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

Layouts typically render another template from the global set. Use `burrow.TemplateExecutorFromContext()` to get a function that executes named templates:

```go
type TemplateExecutor func(r *http.Request, name string, data map[string]any) (template.HTML, error)
```

The executor is injected by the framework's template middleware and handles request-scoped FuncMap cloning automatically.

### Example: Minimal Custom Layout

Here's a complete example of a custom layout with a navbar and footer. First, the layout function:

```go
func appLayout() burrow.LayoutFunc {
    return func(w http.ResponseWriter, r *http.Request, code int, content template.HTML, data map[string]any) error {
        exec := burrow.TemplateExecutorFromContext(r.Context())

        layoutData := maps.Clone(data)
        layoutData["Content"] = content
        layoutData["NavItems"] = burrow.NavItems(r.Context())

        html, err := exec(r, "myapp/layout", layoutData)
        if err != nil {
            return err
        }

        return burrow.HTML(w, code, string(html))
    }
}
```

The corresponding template (`templates/layout.html` in your app):

```html
{{ define "myapp/layout" -}}
<!doctype html>
<html lang="{{ lang }}">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{ .Title }}</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <nav>
        {{ range .NavItems }}
            <a href="{{ .URL }}">{{ .Label }}</a>
        {{ end }}
    </nav>

    <main>{{ .Content }}</main>

    <footer>
        <p>&copy; 2026 My App</p>
    </footer>
</body>
</html>
{{- end }}
```

Wire it up in your server setup:

```go
srv := burrow.NewServer(myapp.New(), /* ... */)
srv.SetLayout(appLayout())
```

Your app must implement `HasTemplates` so that `myapp/layout` is part of the global template set. See [Creating an App](creating-an-app.md#step-6-assemble-the-app) for how to provide template files.

### How the Bootstrap App Does It

Here's how the built-in Bootstrap app implements the same pattern:

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

        return burrow.HTML(w, code, string(html))
    }
}
```

The corresponding template file (`bootstrap/layout`):

```html
{{ define "bootstrap/layout" -}}
<!doctype html>
<html lang="{{ lang }}">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{ .Title }}</title>
    <link rel="stylesheet" href="{{ staticURL "bootstrap/bootstrap.min.css" }}">
    {{ template "bootstrap/theme_script" . }}
    <script defer src="{{ staticURL "bootstrap/bootstrap.bundle.min.js" }}"></script>
    <script defer src="{{ staticURL "htmx/htmx.min.js" }}"></script>
</head>
<body>
    <main class="container py-4">
        {{ .Content }}
    </main>
</body>
</html>
{{- end }}
```

!!! note "No navigation in the base layout"
    The built-in `bootstrap/layout` is intentionally minimal — it provides the HTML shell, CSS/JS assets, and theme switching. If you need a navbar with navigation items, write a custom layout that wraps this one or replaces it entirely. The [tutorial](../tutorial/part3.md) shows how to build a layout with navigation.

## Data Flow in Layout Templates

Layout templates receive data from two sources:

- **Data map entries** (accessed with `.` prefix): `.Content`, `.Title` — these are values your layout function puts into the `layoutData` map before calling `exec()`. Custom layouts can add more (e.g., `.NavItems`, `.Messages`).
- **FuncMap functions** (no `.` prefix): `{{ lang }}`, `{{ staticURL "..." }}`, `{{ csrfToken }}` — these are template functions registered by contrib apps

The layout function is responsible for populating the data map. The Bootstrap layout copies all handler data and adds `Content`:

```go
layoutData := make(map[string]any, len(data)+2)
maps.Copy(layoutData, data)       // copies Title and any other handler data
layoutData["Content"] = content   // the rendered page fragment
```

If your custom layout needs navigation, add it yourself:

```go
layoutData["NavItems"] = burrow.NavItems(r.Context())
```

## Layout Unification

The app layout, auth layout, and admin layout all use the same context key (`burrow.Layout(ctx)`). The framework sets the app layout globally via middleware, while the auth and admin route groups override it with their own layouts. This means any handler can always rely on `burrow.Layout(ctx)` returning the correct layout for the current request.

## Available Context Values

See [Template Functions](../reference/template-functions.md) for the complete list of functions available in templates (e.g. `csrfToken`, `lang`, `t`, `staticURL`, `currentUser`), and [Context Helpers](../reference/context-helpers.md) for Go-level context access (e.g. `burrow.NavItems(ctx)`, `burrow.Layout(ctx)`).
