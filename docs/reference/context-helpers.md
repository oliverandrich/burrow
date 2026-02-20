# Context Helpers

The framework and contrib apps store values in the request context. These helpers read and write those values.

## Core Helpers

Defined in `codeberg.org/oliverandrich/burrow`.

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

### CSRFToken

```go
func CSRFToken(ctx context.Context) string
```

Returns the CSRF token from the context. Returns `""` if not set.

### WithCSRFToken

```go
func WithCSRFToken(ctx context.Context, token string) context.Context
```

Stores a CSRF token in the context.

### Generic Helpers

```go
func WithContextValue(ctx context.Context, key, val any) context.Context
func ContextValue[T any](ctx context.Context, key any) (T, bool)
```

Generic context value helpers. `ContextValue` is a typed getter that returns the value and a boolean indicating whether it was found.

## Auth Helpers

Defined in `codeberg.org/oliverandrich/burrow/contrib/auth`.

!!! note
    Auth helpers use the Echo context (`*echo.Context`), not `context.Context`, because the user is stored in Echo's key-value store rather than the request context.

### GetUser

```go
func GetUser(c *echo.Context) *User
```

Returns the authenticated user from the Echo context, or `nil` if not logged in.

### IsAuthenticated

```go
func IsAuthenticated(c *echo.Context) bool
```

Returns `true` if a user is logged in.

### SetUser

```go
func SetUser(c *echo.Context, user *User)
```

Stores the user in the Echo context. Used internally by the auth middleware.

## Session Helpers

Defined in `codeberg.org/oliverandrich/burrow/contrib/session`.

### Getters

```go
func GetString(c *echo.Context, key string) string
func GetInt64(c *echo.Context, key string) int64
func GetValues(c *echo.Context) map[string]any
```

Read values from the session. Return zero values if the key is missing.

### Setters

```go
func Set(c *echo.Context, key string, value any) error
func Delete(c *echo.Context, key string) error
func Save(c *echo.Context, values map[string]any) error
func Clear(c *echo.Context)
```

Write values to the session. `Set`, `Delete`, and `Save` immediately write the cookie and return an error if the session middleware is not active.

### Testing

```go
func Inject(c *echo.Context, values map[string]any)
```

Sets up session state in the Echo context without the full middleware. Intended for use in tests.

## i18n Helpers

Defined in `codeberg.org/oliverandrich/burrow/contrib/i18n`.

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

## Static Files Helpers

Defined in `codeberg.org/oliverandrich/burrow/contrib/staticfiles`.

### URL

```go
func URL(ctx context.Context, name string) string
```

Returns the content-hashed URL for a static file. Falls back to the original name if the file is not in the manifest or the middleware is not active.
