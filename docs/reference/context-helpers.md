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

### Layout

```go
func Layout(ctx context.Context) LayoutFunc
```

Returns the app layout function from the context. Returns `nil` if no layout is set.

### WithLayout

```go
func WithLayout(ctx context.Context, fn LayoutFunc) context.Context
```

Stores the app layout function in the context. Used internally by the framework middleware.

### Generic Helpers

```go
func WithContextValue(ctx context.Context, key, val any) context.Context
func ContextValue[T any](ctx context.Context, key any) (T, bool)
```

Generic context value helpers. `ContextValue` is a typed getter that returns the value and a boolean indicating whether it was found.

## Admin Helpers

Defined in `codeberg.org/oliverandrich/burrow/contrib/admin`.

### NavItems

```go
func NavItems(ctx context.Context) []NavItem
```

Returns admin navigation items injected by the admin middleware. Returns `nil` if not set.

### WithNavItems

```go
func WithNavItems(ctx context.Context, items []burrow.NavItem) context.Context
```

Stores admin navigation items in the context. Used internally by the admin middleware.

### Layout

```go
func Layout(ctx context.Context) burrow.LayoutFunc
```

Returns the admin layout function from the context. Returns `nil` if no admin layout is set.

### WithLayout

```go
func WithLayout(ctx context.Context, fn burrow.LayoutFunc) context.Context
```

Stores the admin layout function in the context. Used internally by the admin middleware.

## CSRF Helpers

Defined in `codeberg.org/oliverandrich/burrow/contrib/csrf`.

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

Defined in `codeberg.org/oliverandrich/burrow/contrib/auth`.

### GetUser

```go
func GetUser(r *http.Request) *User
```

Returns the authenticated user from the request context, or `nil` if not logged in.

### IsAuthenticated

```go
func IsAuthenticated(r *http.Request) bool
```

Returns `true` if a user is logged in.

### WithUser

```go
func WithUser(ctx context.Context, user *User) context.Context
```

Stores the user in the context. Used internally by the auth middleware.

## Session Helpers

Defined in `codeberg.org/oliverandrich/burrow/contrib/session`.

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
