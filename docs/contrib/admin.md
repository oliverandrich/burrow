# Admin

Admin panel coordinator that discovers and mounts admin views from other apps.

**Package:** `codeberg.org/oliverandrich/burrow/contrib/admin`

**Depends on:** `auth`

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

`admin.New()` uses built-in defaults for the layout and dashboard renderer. Use options to override with custom implementations:

```go
admin.New(admin.WithLayout(myCustomLayout), admin.WithDashboardRenderer(myCustomDashboard))
```

The admin app discovers admin views from other apps via the `HasAdmin` interface. Any app that implements `HasAdmin` gets its routes mounted under `/admin` with auth protection.

## Default Layout

The built-in default layout renders a full admin HTML page with Bootstrap 5 styling, a sidebar navigation, and htmx for SPA-like navigation via `hx-get`/`hx-target`.

The layout reads admin nav items from context and renders them in the sidebar. Static assets are served via the `staticfiles` app using content-hashed URLs.

**Note:** The `bootstrap` app must be registered to serve CSS/JS assets. The admin default layout references static files under the `"bootstrap"` prefix.

## ModelAdmin

The `admin/modeladmin` sub-package provides a generic, Django-style CRUD admin for any Bun model:

```go
import "codeberg.org/oliverandrich/burrow/contrib/admin/modeladmin"

ma := &modeladmin.ModelAdmin[Note]{
    Slug:              "notes",
    DisplayName:       "Note",
    DisplayPluralName: "Notes",
    DB:                cfg.DB,
    ListFields:        []string{"ID", "Title", "CreatedAt"},
    OrderBy:           "created_at DESC",
    PageSize:          25,
    CanCreate:         true,
    CanEdit:           true,
    CanDelete:         true,
}
```

### The `verbose` Struct Tag

ModelAdmin uses the `verbose` struct tag to determine column headers and form labels:

```go
type Note struct {
    ID        int64     `bun:",pk,autoincrement" verbose:"ID"`
    Title     string    `verbose:"Title"`
    Body      string    `verbose:"Body"`
    CreatedAt time.Time `verbose:"Created"`
}
```

If no `verbose` tag is set, the Go field name is used as-is.

### Features

- **List view** with configurable columns, ordering, and offset pagination
- **Search** across text fields
- **Filters** with select dropdowns
- **Row actions** (e.g., retry, cancel, delete) with optional confirmation dialogs
- **Create/edit forms** auto-generated from model struct tags (`verbose` for labels)
- **i18n** — all labels are translatable via `LabelKey` fields and the i18n app
- **HTMX** — list navigation uses `hx-get`/`hx-target` for partial updates

### Row Actions

```go
RowActions: []modeladmin.RowAction{
    {
        Slug:    "retry",
        Label:   "admin-jobs-action-retry",
        Icon:    bsicons.ArrowCounterclockwise(),
        Class:   "btn-outline-success",
        Handler: retryHandler,
        ShowWhen: func(j Job) bool { return j.Status == StatusFailed },
    },
},
```

### Registering in Your App

Implement `HasAdmin` and delegate to the ModelAdmin:

```go
func (a *App) AdminRoutes(r chi.Router) {
    a.notesAdmin.Routes(r)
}

func (a *App) AdminNavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {Label: "Notes", LabelKey: "admin-nav-notes", URL: "/admin/notes", Icon: bsicons.JournalText(), Position: 20},
    }
}
```

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
| `burrow.App` | Required: `Name()`, `Register()` |
| `HasRoutes` | Creates `/admin` group and delegates to `HasAdmin` apps |
| `HasTemplates` | Contributes admin layout and page templates |
| `HasFuncMap` | Contributes admin icon template functions |
| `HasTranslations` | Contributes English and German translations for admin UI |
| `HasDependencies` | Requires `auth` |
