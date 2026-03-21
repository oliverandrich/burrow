# Context Helpers

The framework and contrib apps store values in the request context. These helpers read and write those values.

## Core Helpers

Defined in `github.com/oliverandrich/burrow`.

### NavItems

```go
func NavItems(ctx context.Context) []NavItem
```

Returns navigation items injected by the framework middleware. Returns `nil` if no items are set.

### WithNavItems

```go
func WithNavItems(ctx context.Context, items []NavItem) context.Context
```

Stores navigation items in the context. Used internally by the framework.

### WithAuthChecker

```go
func WithAuthChecker(ctx context.Context, checker AuthChecker) context.Context
```

Stores an `AuthChecker` in the context. The `auth` contrib app calls this automatically in its middleware. The core `navLinks` template function reads it to filter `AuthOnly`/`AdminOnly` nav items. When no `AuthChecker` is set, those items are hidden.

```go
type AuthChecker struct {
    IsAuthenticated func() bool
    IsAdmin         func() bool
}
```

Only needed when using a custom auth system instead of the `auth` contrib app.

### Layout

```go
func Layout(ctx context.Context) string
```

Returns the app layout template name from the context. Returns `""` if no layout is set.

### WithLayout

```go
func WithLayout(ctx context.Context, name string) context.Context
```

Stores the app layout template name in the context. Used internally by the framework middleware.

### Generic Helpers

```go
func WithContextValue(ctx context.Context, key, val any) context.Context
func ContextValue[T any](ctx context.Context, key any) (T, bool)
```

Generic context value helpers. `ContextValue` is a typed getter that returns the value and a boolean indicating whether it was found.

## Admin Helpers

Defined in `github.com/oliverandrich/burrow/contrib/admin`.

### NavGroups

```go
func NavGroups(ctx context.Context) []NavGroup
```

Returns the admin nav groups from the context. Each `NavGroup` contains an app name and its navigation items. Returns `nil` if not set.

> **Deprecated:** `NavGroupsFromContext` will be removed in a future version. Use `NavGroups` instead.

### WithNavGroups

```go
func WithNavGroups(ctx context.Context, groups []NavGroup) context.Context
```

Stores admin nav groups in the context. Used internally by the admin route middleware.

### RequestPath

```go
func RequestPath(ctx context.Context) string
```

Returns the current request path from the context. Used by admin templates to highlight the active sidebar link. Returns `""` if not set.

> **Deprecated:** `RequestPathFromContext` will be removed in a future version. Use `RequestPath` instead.

### WithRequestPath

```go
func WithRequestPath(ctx context.Context, path string) context.Context
```

Stores the current request path in the context. Used internally by the admin route middleware.

!!! note
    The admin layout template name is injected via `burrow.WithLayout(ctx, "admin/layout")` inside the `/admin` route group — there is no separate `admin.Layout` or `admin.WithLayout` helper.

## CSRF Helpers

Defined in `github.com/oliverandrich/burrow/contrib/csrf`.

### Token

```go
func Token(ctx context.Context) string
```

Returns the CSRF token from the context. Returns `""` if not set.

### WithToken

```go
func WithToken(ctx context.Context, token string) context.Context
```

Stores a CSRF token in the context. Used internally by the CSRF middleware.

## Auth Helpers

Defined in `github.com/oliverandrich/burrow/contrib/auth`.

### CurrentUser

```go
func CurrentUser(ctx context.Context) *User
```

Returns the authenticated user from the context, or `nil` if not logged in.

> **Deprecated:** `UserFromContext` will be removed in a future version. Use `CurrentUser` instead.

### IsAuthenticated

```go
func IsAuthenticated(ctx context.Context) bool
```

Returns `true` if a user is logged in.

### WithUser

```go
func WithUser(ctx context.Context, user *User) context.Context
```

Stores the user in the context. Used internally by the auth middleware.

## Session Helpers

Defined in `github.com/oliverandrich/burrow/contrib/session`.

### Getters

```go
func GetString(r *http.Request, key string) string
func GetInt64(r *http.Request, key string) int64
func GetValues(r *http.Request) map[string]any
```

Read values from the session. Return zero values if the key is missing.

### Setters

```go
func Set(w http.ResponseWriter, r *http.Request, key string, value any) error
func Delete(w http.ResponseWriter, r *http.Request, key string) error
func Save(w http.ResponseWriter, r *http.Request, values map[string]any) error
func Clear(w http.ResponseWriter, r *http.Request)
```

Write values to the session. `Set`, `Delete`, and `Save` immediately write the cookie and return an error if the session middleware is not active.

### Testing

```go
func Inject(r *http.Request, values map[string]any) *http.Request
```

Sets up session state in the request context without the full middleware. Intended for use in tests. Returns a new `*http.Request` with the injected values.

## i18n Helpers

Defined in `github.com/oliverandrich/burrow/i18n`.

### Translation

```go
func T(ctx context.Context, key string) string
func TData(ctx context.Context, key string, data map[string]any) string
func TPlural(ctx context.Context, key string, count int) string
```

Translate messages using the localizer from context. Fall back to the message ID if no translation is found.

### Locale

```go
func Locale(ctx context.Context) string
```

Returns the current locale (e.g., `"en"`, `"de"`). Defaults to `"en"`.

### Translating Validation Errors

Validation errors are translated via the `Translate` method on `*burrow.ValidationError`, not a standalone function:

```go
ve.Translate(ctx, i18n.TData)
```

For each `FieldError`, looks up the key `"validation-" + tag` (e.g., `"validation-required"`) with `{"Field": fe.Field, "Param": fe.Param}` as template data. If a translation is found, replaces `Message`; otherwise preserves the original English message.

## Static Files Helpers

Defined in `github.com/oliverandrich/burrow/contrib/staticfiles`.

### URL

```go
func URL(ctx context.Context, name string) string
```

Returns the content-hashed URL for a static file. Falls back to the original name if the file is not in the manifest or the middleware is not active.

## Uploads Helpers

Defined in `github.com/oliverandrich/burrow/contrib/uploads`.

### Storage

```go
func Storage(ctx context.Context) Store
```

Returns the `Store` from the context. Injected by the uploads middleware.

### URL

```go
func URL(ctx context.Context, key string) string
```

Returns the public URL for a storage key. If no storage is in the context, returns the key as-is.

## SSE Helpers

Defined in `github.com/oliverandrich/burrow/contrib/sse`.

### Broker

```go
func Broker(ctx context.Context) *EventBroker
```

Returns the `*EventBroker` from the context. Injected by the SSE middleware.

## Messages Helpers

Defined in `github.com/oliverandrich/burrow/contrib/messages`.

### Get

```go
func Get(ctx context.Context) []Message
```

Returns flash messages from the context. Returns `nil` if no messages are set.

### Add

```go
func Add(w http.ResponseWriter, r *http.Request, level, text string)
```

Adds a flash message to the session. The message will be available on the next request.

## Rate Limit Helpers

Defined in `github.com/oliverandrich/burrow/contrib/ratelimit`.

### RetryAfter

```go
func RetryAfter(ctx context.Context) time.Duration
```

Returns the duration until the client can retry. Set by the rate limit middleware when a request is throttled.
