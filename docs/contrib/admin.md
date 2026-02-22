# Admin

User management panel with CLI commands for promoting users and creating invites.

**Package:** `codeberg.org/oliverandrich/burrow/contrib/admin`

**Depends on:** `auth`

## Setup

```go
import admintpl "codeberg.org/oliverandrich/burrow/contrib/admin/templates"

srv := burrow.NewServer(
    &session.App{},
    auth.New(authRenderer),
    admin.New(admintpl.Layout(), admintpl.DefaultDashboardRenderer()),
    staticfiles.New(myStaticFS), // serves admin + user static files
    // ... other apps
)
```

The parameters accept these forms:

```go
admin.New(admintpl.Layout(), admintpl.DefaultDashboardRenderer()) // batteries-included
admin.New(myCustomLayout, myCustomDashboard)  // custom layout + dashboard
admin.New(nil, nil)                           // no layout, plain text dashboard
```

The admin app discovers admin views from other apps via the `HasAdmin` interface. Any app that implements `HasAdmin` gets its routes mounted under `/admin` with auth protection.

## Default Layout

`admintpl.Layout()` returns a `LayoutFunc` that renders a full HTML page with:

- [Bootstrap 5](https://getbootstrap.com/) — responsive CSS framework
- [Bootstrap Icons](https://icons.getbootstrap.com/) — icon webfont
- [htmx](https://htmx.org/) — for progressive enhancement

The layout reads admin nav items from context and renders them in a `<nav>` element. Static assets are served via the `staticfiles` app using content-hashed URLs.

**Note:** When using `admintpl.Layout()`, the `bootstrap` app must be registered to serve CSS/JS assets. The admin default layout references static files under the `"bootstrap"` prefix.

## Routes

All routes require authentication and admin role:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/admin/users` | List all users |
| GET | `/admin/users/:id` | User detail / edit form |
| POST | `/admin/users/:id` | Update user |
| DELETE | `/admin/users/:id` | Delete user |

## CLI Commands

The admin app contributes three CLI subcommands:

```bash
# Promote a user to admin.
go run ./cmd/server promote alice

# Demote an admin to regular user.
go run ./cmd/server demote alice

# Create an invite and print the registration URL.
go run ./cmd/server create-invite user@example.com
```

To wire up CLI commands, add them to your `cli.Command`:

```go
cmd := &cli.Command{
    Name:     "myapp",
    Flags:    srv.Flags(nil),
    Action:   srv.Run,
    Commands: srv.Registry().AllCLICommands(),
}
```

## Navigation

The admin app contributes a "Users" nav item visible only to admins:

```go
NavItem{
    Label:     "Users",
    URL:       "/admin/users",
    Icon:      "bi bi-people",
    Position:  90,
    AdminOnly: true,
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
| `HasDependencies` | Requires `auth` |
