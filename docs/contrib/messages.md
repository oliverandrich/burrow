# Messages

Flash message support for post-redirect-get feedback.

**Package:** `codeberg.org/oliverandrich/burrow/contrib/messages`
**Depends on:** `session`

## Setup

Register the messages app after session (it depends on session for storage):

```go
srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    i18n.New(),
    messages.New(),
    // ... other apps
)
```

The messages app installs middleware that reads flash messages from the session into the request context and clears them, giving each message a single-request lifetime.

## Adding Messages

Use the convenience helpers inside any handler — typically just before a redirect:

```go
import "codeberg.org/oliverandrich/burrow/contrib/messages"

func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) error {
    // ... create resource ...

    if err := messages.AddSuccess(w, r, "Note created."); err != nil {
        return err
    }
    http.Redirect(w, r, "/notes", http.StatusSeeOther)
    return nil
}
```

Available helpers:

| Helper | Level |
|--------|-------|
| `messages.AddInfo(w, r, text)` | `info` |
| `messages.AddSuccess(w, r, text)` | `success` |
| `messages.AddWarning(w, r, text)` | `warning` |
| `messages.AddError(w, r, text)` | `error` |

For full control, use `messages.Add(w, r, level, text)` with any `messages.Level`.

## Reading Messages

Each `Message` has two fields:

- `Level` — one of `messages.Info`, `messages.Success`, `messages.Warning`, `messages.Error`
- `Text` — the message string

### In Go Code

```go
msgs := messages.Get(r.Context())
for _, msg := range msgs {
    fmt.Printf("%s: %s\n", msg.Level, msg.Text)
}
```

### In Layout Templates

Messages are not injected into templates automatically — your layout function must pass them into the template data. Call `messages.Get(r.Context())` in your `LayoutFunc` and add the result to the data map:

```go
layoutData["Messages"] = messages.Get(r.Context())
```

Then render them in your layout template. Note that the `error` level must be mapped to Bootstrap's `danger` class:

```html
{{ if .Messages -}}
{{ range .Messages -}}
<div class="alert alert-{{ if eq .Level "error" }}danger{{ else }}{{ .Level }}{{ end }} alert-dismissible fade show" role="alert">
    {{ .Text }}
    <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
</div>
{{ end -}}
{{ end -}}
```

## Custom Rendering

If you use a different CSS framework, call `messages.Get(ctx)` directly and map levels to your own classes:

```go
func toastClass(level messages.Level) string {
    switch level {
    case messages.Success: return "toast-success"
    case messages.Warning: return "toast-warning"
    case messages.Error:   return "toast-error"
    default:               return "toast-info"
    }
}
```

## How It Works

The middleware creates a mutable, request-scoped store. `Add()` writes to both the store and the session cookie. `Get()` reads from the store and clears the session cookie to prevent double-display.

### Redirect flow (post-redirect-get)

1. **Handler** calls `messages.Add(w, r, level, text)` — writes to store + session cookie
2. **Redirect** sends the browser to the target page (cookie included)
3. **Middleware** on the next request seeds a new store from the session and clears the session
4. **Template** calls `messages.Get(ctx)` — reads from the store, message appears exactly once

### Same-request flow (HTMX partial)

1. **Handler** calls `messages.Add(w, r, level, text)` — writes to store + session cookie
2. **Template** calls `messages.Get(ctx)` — reads from the store, clears the session cookie
3. Browser receives the response with the cleared cookie — no message persists for the next request

## Testing

Use `session.Inject` to set up session state, then call `messages.Add` and read back with `messages.Get`:

```go
func TestFlashMessage(t *testing.T) {
    rec := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/", nil)
    req = session.Inject(req, map[string]any{})

    err := messages.AddSuccess(rec, req, "Saved")
    require.NoError(t, err)

    values := session.GetValues(req)
    msgs := values["_messages"].([]messages.Message)
    assert.Equal(t, messages.Success, msgs[0].Level)
    assert.Equal(t, "Saved", msgs[0].Text)
}
```

For template tests that need messages in context, use `messages.Inject`:

```go
ctx := messages.Inject(context.Background(), []messages.Message{
    {Level: messages.Success, Text: "Done"},
})
// Render template with ctx
```

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `burrow.HasMiddleware` | Flash message middleware (read from session, inject into context, clear) |
| `burrow.HasDependencies` | Declares dependency on `session` |
