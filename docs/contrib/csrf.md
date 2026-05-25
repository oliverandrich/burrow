# CSRF

Cross-Site Request Forgery protection using `gorilla/csrf`.

**Package:** `github.com/oliverandrich/burrow/contrib/csrf`

**Depends on:** none

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
2. The token is available in templates via the `{{ csrfToken }}` function (provided by `HasRequestFuncMap`)
3. Templates include the token in forms as a hidden field
4. On unsafe requests (POST, PUT, DELETE, PATCH), the middleware validates the submitted token against the cookie

## Using Tokens in Templates

The CSRF app implements `HasRequestFuncMap` and provides two template functions:

| Function | Returns | Description |
|----------|---------|-------------|
| `{{ csrfToken }}` | `string` | The raw CSRF token value |
| `{{ csrfField }}` | `template.HTML` | A complete `<input type="hidden">` element |

Use `csrfField` for the common case — it renders the entire hidden input:

```html
{{ define "notes/create" -}}
<form method="POST" action="/notes">
    {{ csrfField }}
    <input type="text" name="title" placeholder="Title">
    <button type="submit">Create</button>
</form>
{{- end }}
```

Use `csrfToken` when you need just the token value, e.g. for meta tags or JavaScript.

### htmx

The admin layout and `example/notes`'s `app/layout` set `{{ csrfHxHeaders }}` on the `<body>` tag, so all htmx requests inside those layouts include the CSRF token automatically.

If you use a custom layout, add the `csrfHxHeaders` function to your `<body>` tag:

```html
<body{{- csrfHxHeaders }}>
```

This renders `hx-headers='{"X-CSRF-Token":"..."}'` when the csrf app is registered, or nothing when it is not — keeping the HTML clean.

### fetch / XMLHttpRequest

Include a meta tag in your layout so JavaScript can read the token from the DOM:

```html
<meta name="csrf-token" content="{{ csrfToken }}">
```

```javascript
fetch("/api/submit", {
    method: "POST",
    headers: {
        "X-CSRF-Token": document.querySelector('meta[name="csrf-token"]').content,
    },
    body: JSON.stringify(data),
});
```

## Go API

The token is also available in Go code via the context:

```go
import "github.com/oliverandrich/burrow/contrib/csrf"

token := csrf.Token(r.Context())
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

## Exempting Webhook Paths

Cross-origin webhook receivers — Webmention inbound, ActivityPub inbox, payment callbacks — accept POSTs from external services without a CSRF token by design. The csrf app honours a capability interface so the app that owns the route declares the exemption locally:

```go
package webmention

import (
    "github.com/oliverandrich/burrow/contrib/csrf"
)

// Compile-time check.
var _ csrf.ExemptPaths = (*App)(nil)

func (a *App) CSRFExemptPaths() []string {
    return []string{"/webmention"}
}
```

At boot the csrf app walks the registry, collects every `CSRFExemptPaths()` return value, and builds a single matcher. No coordination from `main.go` — adding a new webhook receiver only requires implementing the interface on its app.

**Pattern syntax (minimal by design):**

- `"/webmention"` — exact match (matches `/webmention`, **not** `/webmention/x`).
- `"/inbox/"` — prefix match (trailing slash; matches `/inbox/alice`, `/inbox/bob/feed`, **not** `/inbox`).

No glob or chi-style placeholders. Apps with more complex routing constraints should list each path explicitly or front a prefix-matched subspace and gate further inside their handler.

!!! warning "Use sparingly"
    Each exempt path is a hole in CSRF protection. Limit them to endpoints that legitimately accept off-domain POSTs and validate authenticity by other means (HTTP signatures for Webmention, signed payloads for payment webhooks, etc.).

## Cookie Properties

- `HttpOnly: true` — not accessible from JavaScript
- `Secure` — set automatically when base URL is HTTPS
- `SameSite: Lax` — prevents cross-site cookie sending
- `Path: /` — available on all routes

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()` |
| `Configurable` | CLI flag for auth key |
| `HasMiddleware` | CSRF protection middleware |
| `HasRequestFuncMap` | Provides `csrfToken` and `csrfField` functions to templates |
