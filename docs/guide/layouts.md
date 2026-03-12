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
3. Otherwise, renders the layout template (if set), passing the fragment as `.Content` along with the original data
4. If no layout is set, returns the fragment as-is

This means the same handler automatically supports both full page loads and htmx partial updates.

## Layout Templates

A layout is a **template name** (a string) that refers to a template in the global template set. The layout template receives the rendered page fragment as `.Content` along with the original data map.

When `RenderTemplate` wraps content in a layout, it:

1. Renders the content template to produce an HTML fragment
2. Clones the data map and adds a `Content` key with the rendered fragment
3. Renders the layout template with the combined data

Layout templates access dynamic data (navigation, current user, messages, etc.) via **template functions** — not via data map entries injected by Go code. The framework provides core functions like `navLinks` (filtered, active-state-aware navigation), while contrib apps add request-scoped functions via `HasRequestFuncMap` (e.g., `currentUser`, `csrfToken`).

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
srv.SetLayout("myapp/layout")
```

When `SetLayout()` is called, it takes precedence over design system middleware like `bootstrap`.

If neither approach is used, content renders unwrapped.

## Setting the Auth Layout

Public auth pages (login, register, recovery) typically shouldn't show the full app navbar. By default, `auth.New()` uses a built-in layout template name (`DefaultAuthLayout()` returns `"auth/layout"`) that renders a minimal HTML shell with Bootstrap CSS but no navigation. Authenticated auth routes (`/auth/credentials`, `/auth/recovery-codes`) continue to use the global app layout.

To override the auth layout with a custom template name, use `auth.WithAuthLayout()`:

```go
auth.New(
    auth.WithAuthLayout("myapp/auth-layout"),
)
```

## Setting the Admin Layout

The admin layout is owned by the `admin` package. By default, `admin.New()` uses a built-in layout template name (`DefaultLayout()` returns `"admin/layout"`) and dashboard renderer. To override with a custom template name:

```go
admin.New(admin.WithLayout("myapp/admin-layout"))
```

Pass an empty string for no admin layout.

## Writing a Custom Layout

A layout is simply a template in the global template set. Create a `.html` file in your app's `templates/` directory and set its name via `SetLayout()`.

### Example: Minimal Custom Layout

Create `templates/myapp/layout.html` in your app:

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
        {{ range navLinks }}
            <a href="{{ .URL }}"{{ if .IsActive }} class="active"{{ end }}>
                {{ .Label }}
            </a>
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
srv.SetLayout("myapp/layout")
```

Your app must implement `HasTemplates` so that `myapp/layout` is part of the global template set. See [Creating an App](creating-an-app.md#step-6-assemble-the-app) for how to provide template files.

Note how dynamic data like navigation items is accessed via the `navLinks` template function (provided by the framework) rather than through data map entries. The `navLinks` function automatically filters items by auth state and computes active-link highlighting. The `.Content` key is the only data injected automatically by `RenderTemplate` — it contains the rendered page fragment.

### How the Bootstrap App Does It

The built-in Bootstrap app simply returns a template name:

```go
func Layout() string {
    return "bootstrap/layout"
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
    {{ template "bootstrap/css" . }}
    {{ template "bootstrap/theme_script" . }}
    {{ template "bootstrap/js" . }}
    {{ template "htmx/js" . }}
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

- **Data map entries** (accessed with `.` prefix): `.Content`, `.Title` — `RenderTemplate` clones the handler's data map and adds `Content` (the rendered page fragment) before rendering the layout template. All data passed by the handler is available in the layout.
- **Template functions** (no `.` prefix): `{{ lang }}`, `{{ staticURL "..." }}`, `{{ csrfToken }}`, `{{ navLinks }}`, `{{ currentUser }}`, `{{ messages }}` — these are template functions registered by the framework and contrib apps via `HasFuncMap` and `HasRequestFuncMap`

Dynamic data like navigation items, the current user, and flash messages is provided via **template functions**, not via data map entries. This keeps layouts simple — there is no Go layout function to write or maintain.

## Layout Unification

The app layout, auth layout, and admin layout all use the same context key (`burrow.Layout(ctx)`). The framework sets the app layout globally via middleware, while the auth and admin route groups override it with their own layout template names. This means any handler can always rely on `burrow.Layout(ctx)` returning the correct layout template name for the current request.

## Available Context Values

See [Template Functions](../reference/template-functions.md) for the complete list of functions available in templates (e.g. `csrfToken`, `lang`, `t`, `staticURL`, `currentUser`), and [Context Helpers](../reference/context-helpers.md) for Go-level context access (e.g. `burrow.NavItems(ctx)`, `burrow.Layout(ctx)`).
