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

## Add an Admin Link to the Navbar

Users need a way to get to the admin panel. In `internal/pages/pages.go`, add an admin NavItem:

```go
func (a *App) NavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {Label: "Home", URL: "/", Position: 0},
        {Label: "Admin", URL: "/admin", Position: 100, AdminOnly: true},
    }
}
```

Items with `AdminOnly: true` are automatically hidden from non-admin users. The `navLinks` template function handles the filtering — the `auth` middleware injects an `AuthChecker` into the context, and `navLinks` reads it to decide which items to show.

## Run It

```bash
go mod tidy
go run .
```

Register a user, then promote them to admin using the auth CLI command:

```bash
./polls promote --username your-username
```

Visit `/admin/` to see the dashboard. The auth and jobs apps automatically contribute their own admin views — user management, invite management, and job monitoring are available out of the box.

## What You've Learnt

- **`admin.New()`** — coordinates the admin panel with built-in default layout and dashboard
- **`HasAdmin`** — interface for apps to contribute admin routes and navigation
- **`AdminOnly` nav items** — automatically hidden from non-admin users
- **Built-in admin views** — auth (users, invites) and jobs (queue monitoring) come with admin views by default

!!! tip "Building your own admin views"
    To add custom admin pages for your app, implement `HasAdmin` with `AdminRoutes(r chi.Router)` and `AdminNavItems() []burrow.NavItem`. Write handlers using the same `burrow.Handle` and `burrow.Render` patterns you already know. See the `contrib/auth` and `contrib/jobs` source code for complete examples with search, filters, pagination, and inline forms.

## Next

In [Part 7](part7.md), you'll add HTMX for smooth navigation and infinite scroll pagination.
