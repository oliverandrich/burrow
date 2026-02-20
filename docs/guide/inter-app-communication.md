# Inter-App Communication

Apps can interact with each other through the `Registry` and by declaring dependencies.

## Looking Up an App

Use `Registry.Get()` to retrieve another app by name, then type-assert to access its methods:

```go
func (a *App) Register(cfg *burrow.AppConfig) error {
    authApp, ok := cfg.Registry.Get("auth")
    if !ok {
        return fmt.Errorf("auth app not registered")
    }

    // Type-assert to access the auth app's repository.
    if provider, ok := authApp.(interface{ Repo() *auth.Repository }); ok {
        a.authRepo = provider.Repo()
    }

    return nil
}
```

This pattern is used by the `admin` app to access the auth repository for user management.

## Declaring Dependencies

Implement `HasDependencies` to ensure required apps are registered first:

```go
func (a *App) Dependencies() []string {
    return []string{"auth", "session"}
}
```

If a dependency is missing when `NewServer` processes your app, it panics at startup with a clear error message — a programming mistake caught immediately.

!!! warning "Registration Order"
    Dependencies must appear earlier in the `NewServer()` call:

    ```go
    // Correct: session before auth, auth before notes
    srv := burrow.NewServer(
        &session.App{},
        auth.New(nil),
        notes.New(),
    )

    // Wrong: notes before auth — panics at startup
    srv := burrow.NewServer(
        notes.New(),   // needs "auth"
        auth.New(nil), // not registered yet!
    )
    ```

## Using Auth Context

The auth app sets the current user in the Echo context via middleware. Other apps read it with `auth.GetUser()`:

```go
func (h *Handlers) List(c *echo.Context) error {
    user := auth.GetUser(c)
    if user == nil {
        return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
    }

    notes, err := h.repo.ListByUserID(c.Request().Context(), user.ID)
    // ...
}
```

## Using Session Data

Read and write session values from any app:

```go
import "codeberg.org/oliverandrich/burrow/contrib/session"

// Read a value.
userID := session.GetInt64(c, "user_id")
locale := session.GetString(c, "locale")

// Write a value (immediately writes the cookie).
session.Set(c, "theme", "dark")

// Remove a value.
session.Delete(c, "theme")

// Clear the entire session.
session.Clear(c)
```

## Patterns

**Repository sharing** — expose a `Repo()` method on your app and let other apps access it via type assertion through the registry.

**Middleware guards** — use `auth.RequireAuth()` and `auth.RequireAdmin()` in your route groups to protect endpoints.

**Context values** — read shared data (nav items, CSRF token, locale) from the request context using the provided helpers. See [Context Helpers](../reference/context-helpers.md).
