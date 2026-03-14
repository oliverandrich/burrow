# Error Handling

Burrow provides a unified error handling system that renders styled error pages, supports JSON API responses, and integrates with the template and i18n systems. Error pages are fully customizable through templates.

## How Errors Flow

When a handler returns an error, `Handle()` processes it:

```go
r.Get("/notes/:id", burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
    note, err := repo.Get(r.Context(), chi.URLParam(r, "id"))
    if err != nil {
        return burrow.NewHTTPError(http.StatusNotFound, "note not found")
    }
    return burrow.Render(w, r, http.StatusOK, "notes/detail", map[string]any{
        "Note": note,
    })
}))
```

The error handling chain:

1. **`*HTTPError`** — `Handle()` calls `RenderError(w, r, code, message)`
2. **Any other error** — logged as "unhandled error", rendered as 500
3. **Response already started** — logged, no further action (can't change status code)

Errors with status code >= 500 are logged automatically. 4xx errors are not logged (they're expected client errors).

## RenderError

`RenderError` picks the response format automatically:

- **JSON API requests** (`Accept: application/json`) get a JSON response:
  ```json
  {"error": "note not found", "code": 404}
  ```
- **HTML requests** render the `error/{code}` template (e.g. `error/404`) through the standard `Render` pipeline — with layout wrapping, HTMX fragment support, and i18n

You can also call `RenderError` directly in middleware:

```go
func myMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !authorized(r) {
            burrow.RenderError(w, r, http.StatusForbidden, "forbidden")
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

## Built-in Error Pages

Burrow ships default error templates for these status codes:

| Template      | Status Code | Default Message                              |
|---------------|-------------|----------------------------------------------|
| `error/403`   | 403         | You do not have permission to access this page. |
| `error/404`   | 404         | The page you are looking for does not exist.   |
| `error/405`   | 405         | The request method is not supported for this page. |
| `error/500`   | 500         | An unexpected error occurred. Please try again later. |

The default templates are minimal HTML without any CSS framework. They are designed to be overridden.

## Custom Error Pages

To provide your own error pages, define templates with the same names in your app's template FS. The last `{{ define }}` wins, so app templates override the built-in defaults:

```go
// In your app's templates/ directory:
```

```html
{{ define "error/404" }}
<div class="container text-center py-5">
    <h1 class="display-1">404</h1>
    <p class="lead">{{ .Message }}</p>
    <a href="/" class="btn btn-primary">Back to Home</a>
</div>
{{ end }}
```

Template data available:

| Key       | Type     | Description                     |
|-----------|----------|---------------------------------|
| `.Code`   | `int`    | HTTP status code                |
| `.Message`| `string` | Translated error message        |

Since error pages go through the standard `Render` pipeline, they are wrapped in your layout and have access to all template functions (`navLinks`, `currentUser`, `csrfToken`, `t`, `lang`, etc.).

## Design System Integration

A design system app like `contrib/bootstrap` can override error templates to provide styled pages that match the rest of your application. The override chain is:

1. **Burrow core** — minimal HTML (always present)
2. **Design system app** — styled with CSS framework (e.g. Bootstrap)
3. **Your app** — fully custom (if you need per-app error pages)

Each layer overrides the previous one simply by defining the same template name.

## i18n

Error messages are automatically translated using the `error-{code}` i18n key (e.g. `error-404`). Burrow ships translations for English and German. To add translations for other languages, include keys in your translation files:

```toml
# active.fr.toml
error-403 = "Vous n'avez pas la permission d'accéder à cette page."
error-404 = "La page que vous recherchez n'existe pas."
error-405 = "La méthode de requête n'est pas prise en charge pour cette page."
error-500 = "Une erreur inattendue s'est produite. Veuillez réessayer plus tard."
```

If a translation key is not found, `RenderError` falls back to the original English message passed by the handler.

## Chi NotFound and MethodNotAllowed

Burrow automatically registers custom handlers for Chi's `NotFound` and `MethodNotAllowed` callbacks. Requests to undefined routes or with wrong HTTP methods render styled error pages instead of Chi's default plain-text responses.

This works because the handlers go through `Handle()`, which calls `RenderError()`, which renders the error template with your layout.
