# Core Functions

Functions and types exported by the `burrow` package for use in handlers and apps.

## Handlers

### HandlerFunc

```go
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error
```

An HTTP handler that returns an error. Use `Handle()` to convert it to a standard `http.HandlerFunc`.

### Handle

```go
func Handle(fn HandlerFunc) http.HandlerFunc
```

Converts a `HandlerFunc` into a standard `http.HandlerFunc` with centralized error handling:

- `*HTTPError` — sends the error's status code and message
- Any other error — sends 500 Internal Server Error (original error is logged)
- If the response has already started, the error is logged but no response is written

```go
r.Get("/notes", burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
    return burrow.Text(w, http.StatusOK, "Hello!")
}))
```

### HTTPError

```go
type HTTPError struct {
    Message string
    Code    int
}

func NewHTTPError(code int, message string) *HTTPError
```

An error with an HTTP status code. When returned from a `HandlerFunc`, `Handle()` sends it as the response.

```go
return burrow.NewHTTPError(http.StatusNotFound, "note not found")
```

## Response Helpers

### JSON

```go
func JSON(w http.ResponseWriter, code int, v any) error
```

Writes a JSON response. Sets `Content-Type: application/json`.

```go
return burrow.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
```

### Text

```go
func Text(w http.ResponseWriter, code int, s string) error
```

Writes a plain text response. Sets `Content-Type: text/plain; charset=utf-8`.

```go
return burrow.Text(w, http.StatusOK, "Hello!")
```

### HTML

```go
func HTML(w http.ResponseWriter, code int, s string) error
```

Writes an HTML string response. Sets `Content-Type: text/html; charset=utf-8`.

```go
return burrow.HTML(w, http.StatusOK, "<h1>Hello!</h1>")
```

### Render

```go
func Render(w http.ResponseWriter, r *http.Request, statusCode int, content template.HTML) error
```

Writes pre-rendered `template.HTML` content to the response. Useful for raw HTML output or HTMX fragments that are already rendered.

### Render

```go
func Render(w http.ResponseWriter, r *http.Request, statusCode int, name string, data map[string]any) error
```

Executes a named template and writes the result. Applies automatic layout and HTMX logic:

- **HTMX request** (`HX-Request` header) — renders the fragment only, no layout wrapping
- **Normal request with layout** — renders the layout template from context, passing the fragment as `.Content`
- **Normal request without layout** — renders the fragment only

```go
return burrow.Render(w, r, http.StatusOK, "notes/list", map[string]any{
    "Notes": notes,
})
```

See [Layouts & Rendering](../guide/layouts.md) for details on templates and layout wrapping.

## Request Binding

### Bind

```go
func Bind(r *http.Request, v any) error
```

Parses the request body into a struct and validates it. Supports:

- `application/json` — decoded via `json.Decoder`
- `multipart/form-data` — parsed via `r.ParseMultipartForm`, decoded with `form` struct tags
- `application/x-www-form-urlencoded` — parsed via `r.ParseForm`, decoded with `form` struct tags

Returns a `*ValidationError` when validation fails.

```go
var req struct {
    Title   string `form:"title"   validate:"required"`
    Content string `form:"content"`
}
if err := burrow.Bind(r, &req); err != nil {
    return err
}
```

See [Validation](../guide/validation.md) for validation tags and error handling.

### Validate

```go
func Validate(v any) error
```

Validates a struct using `validate` struct tags. Returns `nil` if `v` is not a struct, has no validate tags, or passes all checks. Returns a `*ValidationError` on failure.

```go
if err := burrow.Validate(myStruct); err != nil {
    // handle validation error
}
```

### ValidationError

```go
type ValidationError struct {
    Errors []FieldError
}
```

Returned by `Bind()` and `Validate()` when validation fails. Contains a slice of `FieldError` values describing each failure. **Not** automatically converted to an HTTP response by `Handle()` — your handler must check for it explicitly.

```go
var ve *burrow.ValidationError
if errors.As(err, &ve) {
    if ve.HasField("email") {
        // highlight the email input
    }
}
```

### FieldError

```go
type FieldError struct {
    Field   string // field name (from form/json tag or Go field name)
    Tag     string // validation tag that failed (e.g., "required")
    Param   string // tag parameter (e.g., "3" for min=3)
    Value   any    // the value that failed
    Message string // human-readable error message
}
```

## Context Helpers

### Layout

```go
func WithLayout(ctx context.Context, name string) context.Context
func Layout(ctx context.Context) string
```

Gets or sets the layout template name in the request context. The framework sets this automatically via middleware. Used by `Render` to wrap content in the named layout template.

### NavItems

```go
func WithNavItems(ctx context.Context, items []NavItem) context.Context
func NavItems(ctx context.Context) []NavItem
```

Gets or sets navigation items in the request context. The framework collects items from all `HasNavItems` apps and injects them via middleware.

### AuthChecker

```go
type AuthChecker struct {
    IsAuthenticated func() bool
    IsAdmin         func() bool
}

func WithAuthChecker(ctx context.Context, checker AuthChecker) context.Context
```

Stores authentication state in the context via closures, allowing the core `navLinks` template function to filter `AuthOnly`/`AdminOnly` items without importing `contrib/auth`. The `auth` contrib app injects this automatically. When no `AuthChecker` is set, `AuthOnly` and `AdminOnly` items are hidden.

### TemplateExecutor

```go
type TemplateExecutor func(r *http.Request, name string, data map[string]any) (template.HTML, error)

func WithTemplateExecutor(ctx context.Context, exec TemplateExecutor) context.Context
func TemplateExec(ctx context.Context) TemplateExecutor
```

Gets or sets the template executor in the request context. The framework injects this automatically. Used internally by `Render`.

### Generic Context Helpers

```go
func WithContextValue(ctx context.Context, key, val any) context.Context
func ContextValue[T any](ctx context.Context, key any) (T, bool)
```

Generic context helpers for storing and retrieving typed values. Used by contrib apps for inter-app communication.

```go
ctx = burrow.WithContextValue(ctx, myKey{}, myValue)
val, ok := burrow.ContextValue[MyType](ctx, myKey{})
```

See [Inter-App Communication](../guide/inter-app-communication.md) for usage patterns.

## Configuration Helpers

### FlagSources

```go
func FlagSources(configSource func(key string) cli.ValueSource, envVar, tomlKey string) cli.ValueSourceChain
```

Builds a `cli.ValueSourceChain` from an environment variable and an optional TOML key. Used by contrib apps to wire up flag sources consistently.

```go
&cli.StringFlag{
    Name:    "my-flag",
    Sources: burrow.FlagSources(configSource, "MY_FLAG", "myapp.flag"),
}
```

### IsLocalhost

```go
func IsLocalhost(host string) bool
```

Returns `true` if the host is a localhost address (`""`, `localhost`, `127.0.0.1`, `::1`, or `*.localhost`). Used by the TLS auto-detection logic.

## Layout

Layouts are template name strings. Use `SetLayout("myapp/layout")` to set the layout template name. `Render` renders the content template, then renders the layout template with `.Content` set to the rendered fragment.

See [Layouts & Rendering](../guide/layouts.md) for details on implementing layouts.

## Navigation

### NavItem

```go
type NavItem struct {
    Label     string
    LabelKey  string        // i18n message ID (used instead of Label when i18n is active)
    URL       string
    Icon      template.HTML // inline SVG, empty for no icon
    Position  int           // sort order (lower = earlier)
    AuthOnly  bool          // only shown to authenticated users
    AdminOnly bool          // only shown to admin users
}
```

See [Navigation](../guide/navigation.md) for positioning and ordering.

### NavLink

```go
type NavLink struct {
    Label    string
    URL      string
    Icon     template.HTML
    IsActive bool
}
```

Template-ready navigation item returned by the `navLinks` template function. `IsActive` is `true` when the current request path matches the item's URL (prefix match, with exact match for `/`). See [Navigation](../guide/navigation.md) for usage.
