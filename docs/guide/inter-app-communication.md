# Inter-App Communication

Apps reach sibling apps through the [`registry` package](../reference/server.md#registry). The registry holds every app the server registered with `NewServer`, in order. Apps receive it as `cfg.Registry` inside their `Configure` method:

```go
func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
    // cfg.Registry is the *registry.Registry held by the server.
    // ...
}
```

`burrow.Registry` is a type alias for `*registry.Registry` — both names refer to the same type.

## Declaring Dependencies

Implement `HasDependencies` to ensure required apps are registered first:

```go
func (a *App) Dependencies() []string {
    return []string{"auth", "session"}
}
```

`registry.Add` panics at startup if a declared dependency is missing — a programming mistake caught immediately. `NewServer` also auto-sorts apps so you can pass them in any order.

A declared dependency is the foundation for the **Hard-Dependency** lookup pattern below: once `Dependencies()` lists a provider, the registry guarantees it is present, and a typed lookup can panic on programmer error instead of returning one.

## Choosing a Lookup Pattern

| Failure mode | Function | When to use |
|---|---|---|
| Panic on missing or wrong type | `registry.MustGet[T](reg)` | The provider is declared in `Dependencies()`. |
| Returns `(T, bool)` | `registry.Get[T](reg)` | The provider may be absent and your code degrades gracefully. |
| Iterate, type-assert against an interface | `registry.Apps(reg)` + loop | Any of several apps might implement the contract you need. |
| Lookup by name, then assert | `registry.GetByName(reg, name)` | You know the exact app name but want a non-typed handle (rare). |

### Hard-Dependency — `registry.MustGet[T]`

Use this when the provider is declared in `Dependencies()`. `MustGet` panics when no app of type `T` is registered, when more than one is, or when the registered app has a different concrete type — all three are programming bugs caught at boot. A downstream project's `pages` app reaching its declared `audit` and `site` providers:

```go
import "github.com/oliverandrich/burrow/registry"

func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
    a.audit = registry.MustGet[*audit.App](cfg.Registry).Repo()
    a.site  = registry.MustGet[*site.App](cfg.Registry)
    return nil
}
```

The type parameter is the lookup key; a wrong type fails at compile time.

### Optional-Service — `registry.Get[T]`

Use this when the provider may not be registered and your code can return `nil` or skip the work. `Get` returns the zero value and `false` when no matching app exists. `sse.BrokerFromRegistry` is the canonical example — it lets a background job publish events when SSE is registered and no-op otherwise:

```go
func BrokerFromRegistry(reg *burrow.Registry) *EventBroker {
    if app, ok := registry.Get[*App](reg); ok {
        return app.Broker()
    }
    return nil
}
```

### Provider-Discovery — `registry.Apps` + interface assertion

Use this when any number of apps might implement the contract you depend on, and you want to find one (or check there's exactly one) without coupling to concrete types. `contrib/admin` uses this to find an `AdminAuth` provider:

```go
import "github.com/oliverandrich/burrow/registry"

// burrow.AdminAuth (defined in burrow.go) exposes RequireAuth /
// RequireStaff / RequireAdmin middleware constructors. contrib/auth
// implements it; a custom auth contrib could too.
func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
    for _, app := range registry.Apps(cfg.Registry) {
        if aa, ok := app.(burrow.AdminAuth); ok {
            a.authMiddleware = aa
            return nil
        }
    }
    return fmt.Errorf("admin: no AdminAuth provider found")
}
```

This pattern survives someone swapping `contrib/auth` for a custom provider — the consumer only depends on the interface, not the implementer.

### Name lookup — `registry.GetByName`

`registry.GetByName(reg, "name")` returns the named app as `burrow.App`. Use it when you genuinely need the name as the lookup key (rare) — for example, a maintenance command that toggles a feature on a specific app instance. `MustGetByName` is the panicking equivalent.

## Using Auth Context

The auth app sets the current user in the request context via middleware. Other apps read it with `auth.CurrentUser[auth.EmptyProfile]()`:

```go
func (a *App) List(w http.ResponseWriter, r *http.Request) error {
    user := auth.CurrentUser[auth.EmptyProfile](r.Context())
    if user == nil {
        return burrow.NewHTTPError(http.StatusUnauthorized, "not authenticated")
    }

    notes, err := a.repo.ListByUserID(r.Context(), user.ID)
    // ...
}
```

Route-level guards live on the auth app itself: `auth.RequireAuth()`, `auth.RequireStaff()`, and `auth.RequireAdmin()`. See [auth guide](../contrib/auth.md).

## Using Session Data

Read and write session values from any app:

```go
import "github.com/oliverandrich/burrow/contrib/session"

userID := session.GetInt64(r, "user_id")
locale := session.GetString(r, "locale")

session.Set(w, r, "theme", "dark")
session.Delete(w, r, "theme")
session.Clear(w, r)
```

## Related

- [Context Helpers](../reference/context-helpers.md) — reading shared data (nav items, CSRF token, locale) from the request context
- [Server reference: Registry](../reference/server.md#registry) — the registry API surface and how it fits the boot sequence
