# Static Files

Content-hashed static file serving with cache-busting URLs, similar to Django's `ManifestStaticFilesStorage`.

**Package:** `codeberg.org/oliverandrich/go-webapp-template/contrib/staticfiles`

## Setup

```go
import "embed"

//go:embed static
var staticFS embed.FS

sfApp := staticfiles.New(staticFS)

srv := core.NewServer(
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

In Templ templates:

```
templ Page() {
    <link rel="stylesheet" href={ staticfiles.URL(ctx, "styles.css") } />
    <script src={ staticfiles.URL(ctx, "app.js") }></script>
}
```

If the file is not found in the manifest, the original name is returned as-is (safe fallback).

## Custom Prefix

By default, files are served at `/static/`. Change it with `WithPrefix`:

```go
sfApp := staticfiles.New(staticFS, staticfiles.WithPrefix("/assets/"))
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

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `core.App` | Required: `Name()`, `Register()` |
| `HasRoutes` | Static file serving route |
| `HasMiddleware` | Context injection and cache header middleware |
