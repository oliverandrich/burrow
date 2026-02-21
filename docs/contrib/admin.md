# Admin

User management panel with CLI commands for promoting users and creating invites.

**Package:** `codeberg.org/oliverandrich/burrow/contrib/admin`

**Depends on:** `auth`

## Setup

```go
srv := burrow.NewServer(
    &session.App{},
    auth.New(authRenderer),
    admin.New(),
    // ... other apps
)
```

The admin app discovers admin views from other apps via the `HasAdmin` interface. Any app that implements `HasAdmin` gets its routes mounted under `/admin` with auth protection.

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
| `HasMiddleware` | Injects admin nav items into the request context |
| `HasDependencies` | Requires `auth` |
