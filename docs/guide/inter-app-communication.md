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

!!! info "Auto-sorting"
    `NewServer` automatically sorts apps by their `HasDependencies` declarations — you can list them in any order. If a dependency is missing entirely, the server panics at startup with a clear error message.

## Using Auth Context

The auth app sets the current user in the request context via middleware. Other apps read it with `auth.UserFromContext()`:

```go
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
    user := auth.UserFromContext(r.Context())
    if user == nil {
        return burrow.NewHTTPError(http.StatusUnauthorized, "not authenticated")
    }

    notes, err := h.repo.ListByUserID(r.Context(), user.ID)
    // ...
}
```

## Using Session Data

Read and write session values from any app:

```go
import "codeberg.org/oliverandrich/burrow/contrib/session"

// Read a value.
userID := session.GetInt64(r, "user_id")
locale := session.GetString(r, "locale")

// Write a value (immediately writes the cookie).
session.Set(w, r, "theme", "dark")

// Remove a value.
session.Delete(w, r, "theme")

// Clear the entire session.
session.Clear(w, r)
```

## Patterns

**Repository sharing** — expose a `Repo()` method on your app and let other apps access it via type assertion through the registry.

**Middleware guards** — use `auth.RequireAuth()` and `auth.RequireAdmin()` in your route groups to protect endpoints.

**Context values** — read shared data (nav items, CSRF token, locale) from the request context using the provided helpers. See [Context Helpers](../reference/context-helpers.md).
