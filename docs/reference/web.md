# web — HTTP handlers, response writers, rendering, validation

The `burrow/web` package holds the HTTP-side primitives every burrow handler reaches for: the error-returning `HandlerFunc` shape, the response writers (`JSON`, `Text`, `HTML`), the `Render` family for HTML templates and HTMX fragments, request binding plus struct-tag validation, and the typed `HTTPError`.

The burrow root re-exports every name as either a type alias or a one-line wrapper, so application code stays on `burrow.Handle`, `burrow.JSON`, `burrow.Render`, `burrow.Bind`, `burrow.Validate`, `burrow.NewHTTPError` etc. Reach into `burrow/web` directly only in test packages or framework-internal code that wants to skip the indirection.

Full signatures live on [pkg.go.dev](https://pkg.go.dev/github.com/oliverandrich/burrow/web). This page is a symbol index.

## Handler shape

| Symbol | Notes |
|---|---|
| `HandlerFunc` | `func(http.ResponseWriter, *http.Request) error` — burrow's error-returning handler shape |
| `Handle(fn)` | Wraps a `HandlerFunc` into an `http.HandlerFunc` with centralized error handling, logging, and HTTPError translation |
| `HTTPError` | Typed handler error — carries Code + Message; pair with `errors.As` to extract |
| `NewHTTPError(code, msg)` | Constructor |

## Response writers

| Symbol | Notes |
|---|---|
| `JSON(w, code, v)` | Marshals `v` as JSON with the right Content-Type |
| `Text(w, code, s)` | Plain text response |
| `HTML(w, code, s)` | Pre-built HTML string — sets Content-Type `text/html; charset=utf-8` |
| `CacheControlImmutable` | The `Cache-Control` value for content-hashed assets (`public, max-age=31536000, immutable`) |

## Render family

| Symbol | Notes |
|---|---|
| `Render(w, r, code, name, data)` | Render a named template; wraps in layout for full-page, returns fragment for HTMX requests |
| `RenderError(w, r, code, msg)` | Render a typed error response — chooses JSON or HTML based on Accept |
| `RenderContent(w, r, code, content, data)` | Same as `Render` but uses pre-built `template.HTML` content instead of a template name |
| `RenderFragment(ctx, name, data)` | Render a named template into `template.HTML` for further composition; uses the context's executor |
| `RenderTemplate(...)` | Deprecated alias for `Render`; kept one release window |
| `ErrNoTemplateExecutor` | Returned when `Render*` runs against a context without an executor |

## Request binding + validation

| Symbol | Notes |
|---|---|
| `Bind(r, &v)` | Parses a request body (JSON, multipart, form-encoded) into `v` and validates struct-tag rules |
| `Validate(v)` | Runs validator on `v` and returns `*ValidationError` on failure |
| `ValidationError` | Carries `[]FieldError`; `errors.As`-friendly; `HasField(name)` helper |
| `FieldError` | One field-level failure — Field, Tag, Param, Message |

See [Routing](../guide/routing.md), [Error Handling](../guide/error-handling.md), and [Validation](../guide/validation.md) for usage patterns and the layout/HTMX interplay.
