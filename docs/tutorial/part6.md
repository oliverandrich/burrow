# Part 6: Admin Panel

In this part you'll add an admin panel so administrators can manage questions directly from a web interface.

**Source code:** [`tutorial/step06/`](https://github.com/oliverandrich/burrow/tree/main/tutorial/step06)

## Add the Admin App

The `admin` contrib app coordinates the admin panel. It provides a dashboard, layout, and route grouping. Apps contribute admin pages by implementing `HasAdmin`.

Update `main.go`:

```go
import (
    "github.com/oliverandrich/burrow/contrib/admin"
)

srv := burrow.NewServer(
    // ... existing apps ...
    polls.New(),
    admin.New(),          // new
)
```

The admin app:

- Provides routes under `/admin/` protected by `auth.RequireAdmin()` middleware
- Collects nav items and routes from all `HasAdmin` apps
- Applies its own top-nav layout with a dashboard at `/admin/` that lists all admin sections as cards

## Optional: an Admin Link in the Navbar

The admin panel is reachable at `/admin/` out of the box, but you can give admins a one-click shortcut by adding a nav item. In `internal/app/app.go`:

```go
func (a *App) NavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {Label: "Home", URL: "/", Position: 0},
        {Label: "Admin", URL: "/admin", Position: 100, AdminOnly: true},
    }
}
```

Items with `AdminOnly: true` are automatically hidden from non-admin users. The `navLinks` template function handles the filtering — the `auth` middleware injects an `AuthChecker` into the context, and `navLinks` reads it to decide which items to show. (The reference code in `tutorial/step06/` ships without this item to keep the diff focused on `admin.New()`.)

## Wire the Auth CLI Subcommands

The auth app contributes CLI subcommands like `promote`, `demote`, and `create-invite` via `HasCLICommands`, but they're only reachable when you wire them into your top-level `cli.Command`. Update `main.go` to add `Commands: srv.CLICommands()`:

```go
cmd := &cli.Command{
    Name:     "polls",
    Usage:    "Polls tutorial application",
    Version:  "0.6.0",
    Flags:    srv.Flags(nil),
    Action:   srv.Run,
    Commands: srv.CLICommands(),
}
```

`srv.CLICommands()` wraps each contrib subcommand so the framework's boot lifecycle (open DB, run `Configure()` on every app) runs before the subcommand's Action fires. Without that wrapping, `promote` would hit an uninitialised auth app and fail.

## Run It

```bash
go mod tidy
go run .
```

Register a user, then promote them to admin (the `promote` subcommand takes the username as a positional argument):

```bash
./polls promote your-username
```

Visit `/admin/` to see the dashboard. The auth app automatically contributes its own admin views — user and invite management are available out of the box. (`contrib/jobs` ships an admin UI too; register `jobs.New()` whenever you want to use it.)

## What You've Learnt

- **`admin.New()`** — coordinates the admin panel with built-in default layout and dashboard
- **`HasAdmin`** — interface for apps to contribute admin routes and navigation
- **`AdminOnly` nav items** — automatically hidden from non-admin users
- **Built-in admin views** — auth contributes users and invites pages out of the box; register `jobs.New()` to add queue monitoring

!!! tip "Building your own admin views"
    To add custom admin pages for your app, implement `HasAdmin` with `AdminRoutes(r chi.Router)` and `AdminNavItems() []burrow.NavItem`. Write handlers using the same `burrow.Handle` and `burrow.Render` patterns you already know. See the `contrib/auth` and `contrib/jobs` source code for complete examples with search, filters, pagination, and inline forms.

## Next

In [Part 7](part7.md), you'll add HTMX for smooth navigation and infinite scroll pagination.
