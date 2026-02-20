# Admin

User management panel with CLI commands for promoting users and creating invites.

**Package:** `codeberg.org/oliverandrich/burrow/contrib/admin`

**Depends on:** `auth`

## Setup

```go
adminApp := admin.New()

srv := burrow.NewServer(
    &session.App{},
    auth.New(authRenderer),
    adminApp,
    // ... other apps
)

// Set a renderer for admin HTML pages (optional).
adminApp.SetHandlers(adminRenderer)
```

## Routes

All routes require authentication and admin role:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/admin/users` | List all users |
| GET | `/admin/users/:id` | User detail |
| POST | `/admin/users/:id/role` | Update user role |

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

## Renderer

The admin app accepts a `Renderer` interface for HTML pages:

```go
type Renderer interface {
    UsersPage(c *echo.Context, users []auth.User) error
    UserDetailPage(c *echo.Context, user *auth.User) error
}
```

Call `adminApp.SetHandlers(renderer)` after `Register()` to enable HTML pages. Without a renderer, routes are not registered.

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `HasRoutes` | Admin user management routes |
| `HasNavItems` | "Users" admin navigation entry |
| `HasCLICommands` | `promote`, `demote`, `create-invite` |
| `HasDependencies` | Requires `auth` |
