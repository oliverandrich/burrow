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

Use `burrow.Render()` to render a named template with data:

```go
func (a *App) List(w http.ResponseWriter, r *http.Request) error {
    notes, err := a.repo.ListAll(r.Context())
    if err != nil {
        return err
    }

    return burrow.Render(w, r, http.StatusOK, "notes/list", map[string]any{
        "Title": "My Notes",
        "Notes": notes,
    })
}
```

The `data` parameter is a `map[string]any`. You don't need to flatten structs into individual map keys — pass them as values and access their fields directly in templates:

```go
return burrow.Render(w, r, http.StatusOK, "articles/detail", map[string]any{
    "Article": article,
    "Related": relatedArticles,
})
```

```html
<h1>{{ .Article.Title }}</h1>
<p>{{ .Article.Description }}</p>

{{ range .Related }}
    <a href="{{ .URL }}">{{ .Title }}</a>
{{ end }}
```

Go's `html/template` can access exported struct fields, methods, and pointer fields — so there's no need to convert models to maps.

`Render` does the following:

1. Executes the named template with the provided data, producing an HTML fragment
2. If the request has an `HX-Request: true` header (htmx), returns the fragment directly — no layout wrapping
3. Otherwise, renders the layout template (if set), passing the fragment as `.Content` along with the original data
4. If no layout is set, returns the fragment as-is

This means the same handler automatically supports both full page loads and htmx partial updates.

## Layout Templates

A layout is a **template name** (a string) that refers to a template in the global template set. The layout template receives the rendered page fragment as `.Content` along with the original data map.

When `Render` wraps content in a layout, it:

1. Renders the content template to produce an HTML fragment
2. Clones the data map and adds a `Content` key with the rendered fragment
3. Renders the layout template with the combined data

Layout templates access dynamic data (navigation, current user, messages, etc.) via **template functions** — not via data map entries injected by Go code. The framework provides core functions like `navLinks` (filtered, active-state-aware navigation), while contrib apps add request-scoped functions via `HasRequestFuncMap` (e.g., `currentUser`, `csrfToken`).

## Setting the App Layout

Set the layout by name in `main.go`:

```go
srv.SetLayout("app/layout")
```

The name refers to a `{{ define "app/layout" }}` block in a template file shipped by one of your apps (typically a small shell app under `internal/app/` that owns the project-level layout — see [Why the shell is in `internal/app/`](tailwind.md#why-the-shell-is-in-internalapp) in the Tailwind guide).

If `SetLayout` is not called, content renders unwrapped (no shell).

## Setting the Auth Layout

Public auth pages (login, register, recovery) typically shouldn't show the full app navbar. By default, `auth.New()` uses `DefaultAuthLayout()` which returns the empty string, meaning auth pages inherit the layout set via `srv.SetLayout`. Authenticated auth routes (`/auth/credentials`, `/auth/recovery-codes`) continue to use the global app layout.

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

## Overriding Contrib Templates

Burrow's template loader uses Go's `html/template` package, which obeys **last-define-wins**: when multiple `{{ define "name" }}` blocks share a name, whichever one is parsed last is the version everyone sees. Templates from each app are parsed in the order the apps were registered with `NewServer`, so the override pattern is simply:

- Define the same template name in your own app's `templates/` directory.
- Make sure your app is registered *after* the contrib whose template you're replacing.

### Example: override the admin dashboard

`contrib/admin` ships `{{ define "admin/dashboard" }}`. To replace it with your own:

```go
// internal/myadmin/app.go
//go:embed templates
var templateFS embed.FS

func (a *App) TemplateFS() fs.FS {
    sub, _ := fs.Sub(templateFS, "templates")
    return sub
}
```

```html
{{ define "admin/dashboard" }}
<section class="...">your custom dashboard markup</section>
{{ end }}
```

And register your app *after* admin:

```go
srv := burrow.NewServer(
    staticApp,
    admin.New(),         // ships admin/dashboard
    myadmin.New(),       // {{ define "admin/dashboard" }} → wins
)
```

If your app comes *before* admin in the slice, admin's parse runs last and your override is silently ignored. Two reliable ways to guarantee the correct order:

**1. List your app after the contrib in `NewServer(...)`:**

```go
srv := burrow.NewServer(
    staticApp,
    admin.New(),
    myadmin.New(),       // explicitly after admin
)
```

**2. Declare the contrib as a dependency** — Burrow's topological sort then puts your app after it regardless of slice order:

```go
func (a *App) Dependencies() []string {
    return []string{"admin"}
}
```

The second form is more robust: it survives a future refactor that reshuffles `NewServer` arguments, and it documents the intent ("I depend on admin's templates being parsed first") in the code that needs the guarantee.

### What you can override

Anything a contrib registers with `{{ define "name" }}` is overridable:

- Layouts (`admin/layout`, `auth/layout`, …)
- Page templates (`admin/dashboard`, `auth/login`, …)
- Partials (`htmx/dialog_script`, `htmx/config`, …)
- Error pages (`error/404`, `error/default`, …)

This is the same mechanism that lets a design system app style error pages — see [Error Handling → Design System Integration](error-handling.md#design-system-integration).

### What this does NOT solve

Contrib templates may include other contrib templates (`{{ template "htmx/js" . }}` etc.). Overriding the *outer* template lets you compose differently, but you can't override the inner template's *internals* unless you override that inner template too. If a contrib's page template inlines markup directly (rather than splitting it into named blocks), the only way to customize that fragment is to redefine the whole page.

This is the price of the override-by-define mechanism. If a contrib feels too monolithic, file an issue requesting it split into smaller `{{ define }}` blocks — usually a quick fix.

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

Note how dynamic data like navigation items is accessed via the `navLinks` template function (provided by the framework) rather than through data map entries. The `navLinks` function automatically filters items by auth state and computes active-link highlighting. The `.Content` key is the only data injected automatically by `Render` — it contains the rendered page fragment.

### How example/notes Does It

`example/notes` ships its layout from `internal/app/templates/app/layout.html` and wires it up as:

```go
srv.SetLayout("app/layout")
```

The layout is a single `<html>` shell — Tailwind CSS link, htmx config, a Tailwind-styled `<nav>` with `navLinks` + user dropdown, an alerts container, `<main>` for `.Content`, and a hidden `<dialog id="modal">` for HTMX-driven dialog flows. See [`example/notes`](https://github.com/oliverandrich/burrow/tree/main/example/notes) for the full implementation.

## Data Flow in Layout Templates

Layout templates receive data from two sources:

- **Data map entries** (accessed with `.` prefix): `.Content`, `.Title` — `Render` clones the handler's data map and adds `Content` (the rendered page fragment) before rendering the layout template. All data passed by the handler is available in the layout.
- **Template functions** (no `.` prefix): `{{ lang }}`, `{{ staticURL "..." }}`, `{{ csrfToken }}`, `{{ navLinks }}`, `{{ currentUser }}`, `{{ messages }}` — these are template functions registered by the framework and contrib apps via `HasFuncMap` and `HasRequestFuncMap`

Dynamic data like navigation items, the current user, and flash messages is provided via **template functions**, not via data map entries. This keeps layouts simple — there is no Go layout function to write or maintain.

## Layout Unification

The app layout, auth layout, and admin layout all use the same context key (`burrow.Layout(ctx)`). The framework sets the app layout globally via middleware, while the auth and admin route groups override it with their own layout template names. This means any handler can always rely on `burrow.Layout(ctx)` returning the correct layout template name for the current request.

## Available Context Values

See [Template Functions](../reference/template-functions.md) for the complete list of functions available in templates (e.g. `csrfToken`, `lang`, `t`, `staticURL`, `currentUser`), and [Context Helpers](../reference/context-helpers.md) for Go-level context access (e.g. `burrow.NavItems(ctx)`, `burrow.Layout(ctx)`).
