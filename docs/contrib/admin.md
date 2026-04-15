# Admin

Admin panel coordinator that discovers and mounts admin views from other apps.

**Package:** `github.com/oliverandrich/burrow/contrib/admin`

**Requires:** an app implementing `burrow.AdminAuth` (e.g., `contrib/auth`)

## Setup

```go
srv := burrow.NewServer(
    session.New(),
    auth.New(),
    admin.New(),
    staticApp, // staticfiles.New(myStaticFS) — returns (*App, error)
    // ... other apps
)
```

`admin.New()` uses built-in defaults for the layout template and dashboard renderer. Use options to override:

```go
admin.New(admin.WithLayout("myapp/admin-layout"), admin.WithDashboardRenderer(myCustomDashboard))
```

The admin app discovers auth middleware via the `AdminAuth` interface and admin views via the `HasAdmin` interface. Any app that implements `HasAdmin` gets its routes mounted under `/admin` with auth protection. The admin app does not import `contrib/auth` directly — any app implementing `AdminAuth` can provide the middleware.

## Default Layout

The built-in default layout renders a full admin HTML page with Bootstrap 5 styling, a sidebar navigation, and htmx for SPA-like navigation via `hx-get`/`hx-target`.

The layout reads admin nav items from context and renders them in the sidebar. Static assets are served via the `staticfiles` app using content-hashed URLs.

**Note:** The `bootstrap` app must be registered to serve CSS/JS assets. The admin default layout references static files under the `"bootstrap"` prefix.

## Building Admin Views

Apps provide admin views by implementing the `HasAdmin` interface and writing handlers directly using `burrow.Handle` and `burrow.Render`. The admin coordinator handles layout, sidebar navigation, and auth middleware — your app only needs to define routes and templates.

```go
func (a *App) AdminRoutes(r chi.Router) {
    r.Get("/notes", burrow.Handle(a.adminListNotes))
    r.Get("/notes/{id}", burrow.Handle(a.adminEditNote))
    r.Post("/notes/{id}", burrow.Handle(a.adminUpdateNote))
    r.Delete("/notes/{id}", burrow.Handle(a.adminDeleteNote))
}

func (a *App) AdminNavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {Label: "Notes", LabelKey: "admin-nav-notes", URL: "/admin/notes", Icon: bsicons.JournalText(), Position: 20},
    }
}
```

Admin handlers follow the same patterns as regular handlers — use Den queries for data access, `burrow.Render` for template rendering, and `htmx.SmartRedirect` for redirects from htmx requests. See the `contrib/auth` and `contrib/jobs` source code for complete examples of admin views with search, filters, pagination, and inline forms.

## Routes

The admin app creates the `/admin` route group with `auth.RequireAuth()` and `auth.RequireAdmin()` middleware, then delegates to each `HasAdmin` app.

The dashboard is available at `GET /admin/`.

## CLI Commands

The CLI subcommands for user management (`promote`, `demote`, `create-invite`) are contributed by the **auth** app via `HasCLICommands`, not by the admin app itself. See [Auth docs](auth.md) for details.

To wire up CLI commands from all apps, add them to your `cli.Command`:

```go
cmd := &cli.Command{
    Name:     "myapp",
    Flags:    srv.Flags(nil),
    Action:   srv.Run,
    Commands: srv.Registry().AllCLICommands(),
}
```

## HasAdmin Interface

Apps contribute admin views by implementing `HasAdmin`:

```go
type HasAdmin interface {
    AdminRoutes(r chi.Router)
    AdminNavItems() []NavItem
}
```

The admin app collects all `HasAdmin` implementations and mounts their routes under `/admin` with `auth.RequireAuth()` and `auth.RequireAdmin()` middleware.

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()` |
| `HasRoutes` | Creates `/admin` group and delegates to `HasAdmin` apps |
| `HasTemplates` | Contributes admin layout and page templates |
| `HasFuncMap` | Contributes admin icon template functions |
| `HasTranslations` | Contributes English and German translations for admin UI |
| `HasDependencies` | Requires `auth` |
