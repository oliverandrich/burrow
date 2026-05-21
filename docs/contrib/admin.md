# Admin

Admin panel coordinator that discovers and mounts admin views from other apps.

**Package:** `github.com/oliverandrich/burrow/contrib/admin`

**Requires:** an app implementing `burrow.AdminAuth` (e.g., `contrib/auth`)

## Setup

```go
srv := burrow.NewServer(
    session.New(),
    auth.New[auth.EmptyProfile](),
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

The built-in default layout renders a full admin HTML page with Tailwind v4 styling, a top navbar (brand on the left, user info on the right), and htmx for `hx-boost`-powered navigation. The dashboard at `/admin/` is the primary navigation surface — each registered admin section appears as a card.

Each admin page renders its own breadcrumb (`<nav><ul class="breadcrumb">…<li aria-current="page">…</ul></nav>`) for back-navigation. There is no persistent sidebar — the cards-on-dashboard pattern keeps the chrome minimal and works the same on desktop and mobile.

Static assets are served via the `staticfiles` app using content-hashed URLs.

**Dependencies:** `staticfiles`, `htmx`, `messages`, `csrf`, `auth` — all five must be registered. The layout uses `{{ csrfToken }}`, `{{ csrfHxHeaders }}`, `{{ currentUser }}`, and admin's `Configure` needs an `AdminAuth` provider (supplied by `contrib/auth`).

## Building Admin Views

Apps provide admin views by implementing the `HasAdmin` interface and writing handlers directly using `burrow.Handle` and `burrow.Render`. The admin coordinator handles layout, dashboard cards, and auth middleware — your app only needs to define routes and templates.

```go
func (a *App) AdminRoutes(r chi.Router) {
    r.Get("/notes", burrow.Handle(a.adminListNotes))
    r.Get("/notes/{id}", burrow.Handle(a.adminEditNote))
    r.Post("/notes/{id}", burrow.Handle(a.adminUpdateNote))
    r.Delete("/notes/{id}", burrow.Handle(a.adminDeleteNote))
}

func (a *App) AdminNavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {Label: "Notes", URL: "/admin/notes", Icon: "notes/icon_journal_text", Position: 20},
    }
}
```

Admin handlers follow the same patterns as regular handlers — use Den queries for data access, `burrow.Render` for template rendering, and `htmx.SmartRedirect` for redirects from htmx requests. The admin contrib ships a shared `admin/pagination` template (handlers pass `RawQuery: r.URL.RawQuery` so filters/search persist across pages). See the `contrib/auth` and `contrib/jobs` source code for complete examples of admin views with search, filters, pagination, and inline forms.

## Routes

The admin app creates the `/admin` route group with `auth.RequireAuth()` and `auth.RequireStaff()` — any logged-in user with the `staff` or `admin` role passes the frame gate. Apps that ship admin-only routes wrap them inside their own `AdminRoutes` with `auth.RequireAdmin()`:

```go
func (a *App) AdminRoutes(r chi.Router) {
    // Open to any staff member.
    r.Get("/posts", burrow.Handle(a.adminListPosts))
    r.Get("/drafts/{id}", burrow.Handle(a.adminEditDraft))

    // Admin-only group.
    r.Group(func(r chi.Router) {
        r.Use(auth.RequireAdmin())
        r.Get("/settings", burrow.Handle(a.adminSettings))
    })
}
```

The dashboard at `GET /admin/` lists every nav item contributed via `AdminNavItems()`, filtered per request: `NavItem.AdminOnly: true` hides the entry from non-admin staff. Groups whose items all become hidden disappear from the dashboard too.

## CLI Commands

The CLI subcommands for user management (`set-role`, `create-invite`) are contributed by the **auth** app via `HasCLICommands`, not by the admin app itself. See [Auth docs](auth.md) for details.

To wire up CLI commands from all apps, add them to your `cli.Command` via `srv.CLICommands()`:

```go
cmd := &cli.Command{
    Name:     "myapp",
    Flags:    srv.Flags(nil),
    Action:   srv.Run,
    Commands: srv.CLICommands(),
}
```

`srv.CLICommands()` wraps each subcommand's `Action` so the framework's boot lifecycle (database open, `Configure()` on every app) runs before the subcommand fires. The raw `srv.Registry().AllCLICommands()` skips that step and is wrong for contrib subcommands that depend on configured state (e.g. `auth set-role`).

## HasAdmin Interface

Apps contribute admin views by implementing `HasAdmin`:

```go
type HasAdmin interface {
    AdminRoutes(r chi.Router)
    AdminNavItems() []NavItem
}
```

The admin app collects all `HasAdmin` implementations and mounts their routes under `/admin` with `auth.RequireAuth()` + `auth.RequireStaff()` — the frame is staff-gated. Routes that must stay admin-only are the contributing app's responsibility: wrap them in a sub-group with `auth.RequireAdmin()` and tag the matching `AdminNavItems()` entries with `AdminOnly: true` so non-admin staff don't see them in the dashboard.

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()` |
| `HasRoutes` | Creates `/admin` group and delegates to `HasAdmin` apps |
| `HasTemplates` | Contributes admin layout and page templates |
| `HasFuncMap` | Contributes admin icon template functions |
| `HasTranslations` | Contributes English and German translations for admin UI |
| `HasStaticFiles` | Ships `admin.css` (admin-shell polish: action-row alignment, dashboard card link styling, form-footer flex) |
| `HasDependencies` | Requires `staticfiles`, `htmx`, `messages`, `csrf`, `auth` |
