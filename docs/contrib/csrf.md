# CSRF

Cross-Site Request Forgery protection using `gorilla/csrf`.

**Package:** `codeberg.org/oliverandrich/burrow/contrib/csrf`

## Setup

```go
srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    // ... other apps
)
```

The CSRF app provides middleware that protects POST/PUT/DELETE/PATCH requests using the double-submit cookie pattern. GET, HEAD, OPTIONS, and TRACE requests pass through without validation.

## How It Works

1. On every request, the middleware sets a CSRF cookie and generates a masked token
2. The token is stored in the request context via `csrf.WithToken()`
3. Templates read it with `csrf.Token(ctx)` and include it in forms
4. On unsafe requests (POST, PUT, DELETE, PATCH), the middleware validates the submitted token against the cookie

## Using Tokens in Templates

The token is available in any Templ component via the context:

```go
// In a Templ template
templ MyForm() {
    <form method="POST" action="/submit">
        <input type="hidden" name="gorilla.csrf.Token" value={ csrf.Token(ctx) }/>
        <button type="submit">Submit</button>
    </form>
}
```

Alternatively, send the token in the `X-CSRF-Token` HTTP header for AJAX requests:

```javascript
fetch("/api/submit", {
    method: "POST",
    headers: {
        "X-CSRF-Token": document.querySelector('meta[name="csrf-token"]').content,
    },
    body: JSON.stringify(data),
});
```

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--csrf-key` | `CSRF_KEY` | auto-generated | 32-byte hex auth key |

!!! warning "CSRF Key"
    If no key is provided, one is auto-generated and logged to stdout. Tokens will not persist across server restarts. For production, always set `CSRF_KEY`.

    Generate a key:
    ```bash
    openssl rand -hex 32
    ```

## Cookie Properties

- `HttpOnly: true` — not accessible from JavaScript
- `Secure` — set automatically when base URL is HTTPS
- `SameSite: Lax` — prevents cross-site cookie sending
- `Path: /` — available on all routes

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `Configurable` | CLI flag for auth key |
| `HasMiddleware` | CSRF protection middleware |
