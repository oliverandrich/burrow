# Session

Cookie-based session management using `gorilla/securecookie`.

**Package:** `codeberg.org/oliverandrich/burrow/contrib/session`

**Depends on:** none

## Setup

```go
srv := burrow.NewServer(
    session.New(),
    // ... other apps
)
```

The session app provides middleware that automatically parses and manages session cookies on every request.

## Reading Values

```go
import "codeberg.org/oliverandrich/burrow/contrib/session"

// Get a string value.
locale := session.GetString(r, "locale")

// Get an int64 value.
userID := session.GetInt64(r, "user_id")

// Get all values as a map.
values := session.GetValues(r)
```

All getters return zero values if the key is missing or the session is empty.

## Writing Values

```go
// Set a single value (writes the cookie immediately).
session.Set(w, r, "theme", "dark")

// Replace all session values at once.
session.Save(w, r, map[string]any{
    "user_id": int64(42),
    "role":    "admin",
})

// Remove a single key.
session.Delete(w, r, "theme")

// Clear the entire session (writes a deletion cookie).
session.Clear(w, r)
```

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--session-cookie-name` | `SESSION_COOKIE_NAME` | `_session` | Cookie name |
| `--session-max-age` | `SESSION_MAX_AGE` | `604800` (7 days) | Max age in seconds |
| `--session-hash-key` | `SESSION_HASH_KEY` | auto-generated | 32-byte hex key for signing |
| `--session-block-key` | `SESSION_BLOCK_KEY` | (none) | 32-byte hex key for encryption |

!!! warning "Session Keys"
    If no hash key is provided, one is auto-generated and logged to stdout. Sessions will not persist across server restarts. For production, always set `SESSION_HASH_KEY`.

    Generate a key:
    ```bash
    openssl rand -hex 32
    ```

## Cookie Properties

- `HttpOnly: true` — not accessible from JavaScript
- `Secure` — set automatically when base URL is HTTPS
- `SameSite: Lax` — CSRF protection
- `Path: /` — available on all routes

## Testing

Use `session.Inject()` to set up session state in tests without the full middleware:

```go
func TestMyHandler(t *testing.T) {
    req := httptest.NewRequest(http.MethodGet, "/", nil)
    rec := httptest.NewRecorder()

    // Inject session values.
    req = session.Inject(req, map[string]any{
        "user_id": int64(1),
    })

    // Call your handler.
    err := myHandler(rec, req)
    assert.NoError(t, err)
}
```

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `Configurable` | CLI flags for cookie name, max age, keys |
| `HasMiddleware` | Session parsing middleware |
