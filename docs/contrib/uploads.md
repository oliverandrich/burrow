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
    storage := uploads.StorageFromContext(r.Context())

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

## Storage Interface

The uploads app uses a `Storage` interface, making the backend pluggable:

```go
type Storage interface {
    Store(ctx context.Context, file io.Reader, opts StoreOptions) (key string, err error)
    Delete(ctx context.Context, key string) error
    Open(ctx context.Context, key string) (io.ReadCloser, error)
    URL(key string) string
}
```

The built-in `LocalStorage` stores files on the local filesystem with content-hashed filenames for deduplication.

## Context Middleware

The uploads app injects the `Storage` backend and allowed MIME types into every request context via middleware:

```go
storage := uploads.StorageFromContext(r.Context())
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
| `--upload-dir` | `UPLOAD_DIR` | `data/uploads` | Directory for uploaded files |
| `--upload-url-prefix` | `UPLOAD_URL_PREFIX` | `/uploads/` | URL prefix for serving files |
| `--upload-allowed-types` | `UPLOAD_ALLOWED_TYPES` | (all) | Comma-separated allowed MIME types |

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `Configurable` | Upload directory, URL prefix, and allowed types flags |
| `HasMiddleware` | Injects storage and allowed types into context |
| `HasRoutes` | Serves uploaded files at the URL prefix |
