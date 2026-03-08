# Static Files

Content-hashed static file serving with cache-busting URLs, similar to Django's `ManifestStaticFilesStorage`.

**Package:** `codeberg.org/oliverandrich/burrow/contrib/staticfiles`

**Depends on:** none

## Setup

```go
import "embed"

//go:embed static
var staticFS embed.FS

sfApp, err := staticfiles.New(staticFS)
if err != nil {
    log.Fatal(err)
}

srv := burrow.NewServer(
    sfApp,
    // ... other apps
)
```

## How It Works

1. At startup, the app walks the embedded filesystem and computes a SHA-256 hash for each file
2. Files are served under hashed URLs: `styles.css` becomes `styles.a1b2c3d4.css`
3. Hashed URLs get `Cache-Control: public, max-age=31536000, immutable` (1 year)
4. Non-hashed URLs get `Cache-Control: no-cache, no-store, must-revalidate`

## Generating URLs

Use `staticfiles.URL()` in templates to resolve hashed paths:

```go
url := staticfiles.URL(ctx, "styles.css")
// "/static/styles.a1b2c3d4.css"
```

In HTML templates (via `HasFuncMap`):

```html
{{ define "app/layout" -}}
<link rel="stylesheet" href="{{ staticURL "styles.css" }}">
<script src="{{ staticURL "app.js" }}" defer></script>
{{- end }}
```

If the file is not found in the manifest, the original name is returned as-is (safe fallback).

## Custom Prefix

By default, files are served at `/static/`. Change it with `WithPrefix`:

```go
sfApp, err := staticfiles.New(staticFS, staticfiles.WithPrefix("/assets/"))
```

## File Organization

```
static/
├── styles.css
├── app.js
├── images/
│   └── logo.png
└── fonts/
    └── inter.woff2
```

All files are walked recursively. The manifest maps original paths to hashed paths:

| Original | Hashed |
|----------|--------|
| `styles.css` | `styles.a1b2c3d4.css` |
| `images/logo.png` | `images/logo.e5f6a7b8.png` |

## App-Contributed Static Files

Contrib apps can contribute their own CSS/JS by implementing `HasStaticFiles`:

```go
//go:embed static
var adminStaticFS embed.FS

func (a *App) StaticFS() (string, fs.FS) {
    sub, _ := fs.Sub(adminStaticFS, "static")
    return "admin", sub
}
```

The `staticfiles` app automatically discovers all `HasStaticFiles` implementations during `Register()` and serves their files under the declared prefix:

```
/static/admin/admin.a1b2c3d4.css
/static/admin/admin.e5f6a7b8.js
```

Generate URLs with the prefix included:

```go
staticfiles.URL(ctx, "admin/admin.css")
// "/static/admin/admin.a1b2c3d4.css"
```

Files from all sources get the same content-hashing and cache headers.

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `HasRoutes` | Static file serving route |
| `HasMiddleware` | Context injection and cache header middleware |
| `HasFuncMap` | Provides `staticURL` template function |
