# Uploads

Pluggable file upload storage with a local filesystem backend and HTTP serving.

**Package:** `github.com/oliverandrich/burrow/contrib/uploads`

**Depends on:** none

## Setup

```go
srv := burrow.NewServer(
    uploads.New(),
    // ... other apps
)
```

With options:

```go
uploads.New(
    uploads.WithBaseDir("data"),                                    // uploads stored in data/uploads/
    uploads.WithURLPrefix("/uploads/"),                             // served at /uploads/*
    uploads.WithAllowedTypes("image/jpeg", "image/png", "image/webp"), // restrict MIME types
)
```

## Storing Files

Use `uploads.StoreFile()` in a handler to extract and store a file from a multipart form:

```go
import "github.com/oliverandrich/burrow/contrib/uploads"

func (h *Handlers) UploadAvatar(w http.ResponseWriter, r *http.Request) error {
    storage := uploads.Storage(r.Context())

    key, err := uploads.StoreFile(r, "avatar", storage, uploads.StoreOptions{
        Prefix:       "avatars",
        AllowedTypes: []string{"image/jpeg", "image/png"},
        MaxSize:      5 << 20, // 5 MB
    })
    if err != nil {
        return err
    }

    // key is e.g. "avatars/a1b2c3d4.jpg"
    url := storage.URL(key) // e.g. "/uploads/avatars/a1b2c3d4.jpg"

    // Save url to your model...
    return nil
}
```

### StoreOptions

| Field | Type | Description |
|-------|------|-------------|
| `Prefix` | `string` | Subdirectory, e.g. `"avatars"` |
| `Filename` | `string` | Original filename (for extension); defaults to upload filename |
| `AllowedTypes` | `[]string` | Allowed MIME types; empty = use app default or allow all |
| `MaxSize` | `int64` | Per-file limit in bytes; 0 = no limit |

## Deleting Files

Remove a file from storage by its key:

```go
err := storage.Delete(ctx, key)
```

`Delete` is idempotent — it does not return an error if the file does not exist.

Typical handler pattern combining database and storage cleanup:

```go
func (h *Handlers) DeleteAvatar(w http.ResponseWriter, r *http.Request) error {
    storage := uploads.Storage(r.Context())

    user := getUser(r)
    oldKey := user.AvatarKey

    user.AvatarKey = ""
    if _, err := h.db.NewUpdate().Model(user).Column("avatar_key").Exec(r.Context()); err != nil {
        return err
    }

    return storage.Delete(r.Context(), oldKey)
}
```

## Storage Interface

The uploads app uses a `Store` interface, making the backend pluggable:

```go
type Store interface {
    Store(ctx context.Context, file io.Reader, opts StoreOptions) (key string, err error)
    Delete(ctx context.Context, key string) error
    Open(ctx context.Context, key string) (io.ReadCloser, error)
    URL(key string) string
}
```

The built-in `LocalStorage` stores files on the local filesystem with content-hashed filenames for deduplication.

## Custom Storage Backends

To implement a custom backend (e.g., S3, GCS), implement the four methods of the `Store` interface:

```go
type S3Storage struct { /* ... */ }

func (s *S3Storage) Store(ctx context.Context, file io.Reader, opts uploads.StoreOptions) (string, error) { /* ... */ }
func (s *S3Storage) Delete(ctx context.Context, key string) error { /* ... */ }
func (s *S3Storage) Open(ctx context.Context, key string) (io.ReadCloser, error) { /* ... */ }
func (s *S3Storage) URL(key string) string { /* ... */ }
```

The built-in `LocalStorage` provides some useful properties you may want to replicate:

- **Content-hashed filenames** — files are stored under a SHA-256 hash prefix, so identical uploads produce the same key (deduplication).
- **Atomic writes** — files are written to a temporary file first, then renamed into place. This prevents serving partial uploads.
- **Idempotent storage** — if a file with the same content hash already exists, the write is skipped.

## Context Middleware

The uploads app injects the `Storage` backend and allowed MIME types into every request context via middleware:

```go
storage := uploads.Storage(r.Context())
```

## Using URLs in Templates

The `uploads.URL()` context helper returns the public URL for a storage key. If no storage is in the context, it returns the key as-is — making it safe for use in templates:

```go
uploads.URL(ctx, key) // e.g. "/uploads/avatars/a1b2c3d4.jpg"
```

In a template:

```html
<img src="{{ uploadURL .AvatarKey }}" alt="Avatar">
```

To make this work, add `uploadURL` to your app's template FuncMap:

```go
func (a *App) FuncMap() template.FuncMap {
    return template.FuncMap{
        "uploadURL": func() string { return "" }, // placeholder, overridden per request
    }
}

func (a *App) RequestFuncMap(r *http.Request) template.FuncMap {
    return template.FuncMap{
        "uploadURL": func(key string) string {
            return uploads.URL(r.Context(), key)
        },
    }
}
```

## File Serving

Uploaded files are served at the configured URL prefix (default: `/uploads/`) with aggressive caching:

```
Cache-Control: public, max-age=31536000, immutable
```

Since filenames are content-hashed, files are effectively immutable — a changed file gets a new hash and therefore a new URL.

## Errors

| Error | Description |
|-------|-------------|
| `uploads.ErrTypeNotAllowed` | File MIME type not in allowed list |
| `uploads.ErrFileTooLarge` | File exceeds `MaxSize` |
| `uploads.ErrEmptyFile` | Uploaded file is empty |
| `uploads.ErrMissingField` | Form field not found in request |

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--uploads-dir` | `UPLOADS_DIR` | `data/uploads` | Directory for uploaded files |
| `--uploads-url-prefix` | `UPLOADS_URL_PREFIX` | `/uploads/` | URL prefix for serving files |
| `--uploads-allowed-types` | `UPLOADS_ALLOWED_TYPES` | (all) | Comma-separated allowed MIME types |

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `Configurable` | Upload directory, URL prefix, and allowed types flags |
| `HasMiddleware` | Injects storage and allowed types into context |
| `HasRoutes` | Serves uploaded files at the URL prefix |
