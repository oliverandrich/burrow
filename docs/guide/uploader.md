# Uploader

HTTP-layer helpers for file uploads built on [`den.Storage`](https://den-odm.readthedocs.io/guide/attachments/):
an `Uploader` service that validates and persists multipart uploads,
a `Mount` helper that registers the serving route at the Storage's
URL prefix, and a raw `ServeHandler` for hand-routed serving.

**Package:** `github.com/oliverandrich/burrow/uploader`

**Depends on:** `github.com/oliverandrich/den` for `den.Storage` and
`document.Attachment`.

!!! info "Not a contrib app"
    `uploader` is a plain package, not a `burrow.App`. The domain app
    that owns the upload feature holds the `Uploader`, registers the
    serving route via `uploader.Mount`, and defines the POST handler
    that calls `Uploader.Store` — so `uploader` never sees `cfg` or
    `cmd` and owns no CLI flags, no middleware, no context injection.
    Template rendering of attachment URLs is handled globally via the
    built-in [`mediaURL`](#building-urls-in-templates) template
    function — no per-app `FuncMap` required.

## Setup

Burrow constructs the Storage from two CLI flags during `srv.Run`,
mirrors the `--database-dsn` mechanism. There is **nothing to wire in
`main.go`** — attachments just work:

| Flag | Env | Default | Description |
|---|---|---|---|
| `--storage-dsn` | `STORAGE_DSN` | `file:///data/media` | Backend DSN. Schemes: `file://` (`file:///relative` or `file:////absolute`, SQLAlchemy-style). Empty string disables Storage. |
| `--media-url-prefix` | `MEDIA_URL_PREFIX` | `/media/` | Public URL prefix for locally served attachments. |

```go
// main.go — identical to any Burrow app
func main() {
    srv := burrow.NewServer(&blogApp{})
    srv.SetLayout(bootstrap.NavLayout())

    cmd := &cli.Command{
        Flags:  srv.Flags(nil),
        Action: srv.Run,
    }
    _ = cmd.Run(context.Background(), os.Args)
}
```

The domain app reads `cfg.DB` in `Configure` and builds its Uploader
and serving route from there:

```go
// blog.go — your domain app
type blogApp struct {
    db     *den.DB
    upload *uploader.Uploader
}

func (a *blogApp) Name() string { return "blog" }

func (a *blogApp) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
    a.db = cfg.DB
    a.upload = uploader.NewUploader(cfg.DB) // panics if no Storage
    return nil
}

func (a *blogApp) Routes(r chi.Router) {
    uploader.Mount(r, a.db.Storage()) // registers "/media/*" serving route
    r.Post("/upload", a.handleUpload)
}
```

`Configure` receives the configured `*den.DB` via `burrow.AppConfig`
— Burrow has already installed the Storage on it via `den.WithStorage`
before `Configure` runs. `Routes` receives the `chi.Router` during
server bootstrap.

`NewUploader` panics if the `*den.DB` has no Storage — that state
always indicates a setup bug (typically `--storage-dsn=""` while the
app expects uploads), so a loud failure at startup is the right
behavior.

### Disabling Storage

An app that does not need attachments can switch the backend off:

```bash
STORAGE_DSN='' ./myapp
```

With no Storage, `cfg.DB.Storage()` returns `nil`, `mediaURL` is not
registered in the template func map, and any call to
`uploader.NewUploader(db)` panics.

## Storing Files

`Uploader.Store` parses multipart, validates, streams to the Storage,
and returns a [`document.Attachment`](https://den-odm.readthedocs.io/guide/attachments/)
ready to assign onto a Den document field.

### IS-a-file pattern (`document.Attachment` embedded)

Use this when the domain record IS a file — `Media`, `Avatar`, `Cover`.

```go
type Media struct {
    document.Base
    document.Attachment
    AltText string `json:"alt_text,omitempty"`
}

func (a *blogApp) UploadMedia(w http.ResponseWriter, r *http.Request) error {
    att, err := a.upload.Store(r, "file", uploader.StoreOptions{
        AllowedTypes: []string{"image/jpeg", "image/png"},
        MaxSize:      5 << 20, // 5 MB
    })
    if err != nil {
        return err
    }

    media := &Media{Attachment: att, AltText: r.FormValue("alt")}
    if err := den.Insert(r.Context(), a.db, media); err != nil {
        // Bytes were stored but the record did not land — clean up to
        // avoid an orphan. The Delete error is best-effort.
        _ = a.upload.Storage().Delete(r.Context(), att)
        return err
    }
    return nil
}
```

### HAS-file pattern (named Attachment field)

Use this when the domain record HAS a file — `Post.Hero`,
`Product.Thumbnail`.

```go
type Post struct {
    document.Base
    Title string              `json:"title"`
    Hero  document.Attachment `json:"hero"`
}

func (a *blogApp) UploadHero(w http.ResponseWriter, r *http.Request) error {
    att, err := a.upload.Store(r, "hero", uploader.StoreOptions{
        AllowedTypes: []string{"image/"}, // prefix match — any image subtype
        MaxSize:      10 << 20,
    })
    if err != nil {
        return err
    }

    post.Hero = att
    if err := den.Update(r.Context(), a.db, post); err != nil {
        _ = a.upload.Storage().Delete(r.Context(), att)
        return err
    }
    return nil
}
```

### StoreOptions

| Field | Type | Description |
|---|---|---|
| `AllowedTypes` | `[]string` | MIME allow-list (empty = all). Prefix match: `"image/"` accepts any image subtype, `"text/plain"` matches `"text/plain; charset=utf-8"` |
| `MaxSize` | `int64` | Per-file upper bound in bytes (0 = no limit). Enforced mid-stream — no partial file is persisted if the limit is hit |
| `Filename` | `string` | Original filename; used as extension fallback when the MIME type has no mapping. Populated from the multipart header if left empty |

!!! warning "Orphan bytes on Insert failure"
    `Store` persists bytes synchronously. If the subsequent
    `den.Insert` or `den.Update` fails, the bytes are on disk with
    no record pointing at them. The canonical fix is the
    `_ = uploader.Storage().Delete(ctx, att)` call shown above — always
    pair `Store` with a compensating `Delete` in the record-save error
    path.

    Process crashes between `Store` and `Insert` still produce orphans.
    An automated offline sweeper is planned for a later release; until
    then, operators can periodically diff `Storage`-enumerated paths
    against every `StoragePath` in the DB and remove unreferenced
    entries older than a grace period.

## Deleting Files

Remove a file by its Attachment:

```go
err := db.Storage().Delete(ctx, att)
```

Delete is idempotent — missing paths are not errors. If the file
belongs to a hard-deleted Den document, Den's
[hard-delete attachment cascade](https://den-odm.readthedocs.io/guide/attachments/#hard-delete-cascade)
removes the bytes automatically; no handler code required.

## Serving Files

`uploader.Mount` reads the URL prefix from the Storage and registers
a serving handler at that prefix — the prefix is written **once** (at
Storage construction) and never retyped:

```go
uploader.Mount(r, db.Storage())
// registers "/media/*" → streams from the Storage
```

Files come back with long-lived caching
(`Cache-Control: public, max-age=31536000, immutable`). Because
filenames are content-hashed, files are effectively immutable: a
changed file gets a new hash and therefore a new URL, so the cache is
safe.

Mount routes through `Storage.Open` rather than reading the filesystem
directly, so it works with any `den.Storage`. Remote backends that
don't implement `URLPrefix() string` (S3, GCS, CDN — they return
absolute URLs) are skipped; the backend or CDN serves those URLs
directly.

### Hand-mounted serving

`uploader.ServeHandler(s)` is the underlying `http.Handler`. Use it
directly when `Mount` doesn't fit — for example a custom routing
prefix unrelated to the Storage's own URL:

```go
r.Mount("/assets", http.StripPrefix("/assets", uploader.ServeHandler(fs)))
```

`ServeHandler` treats `r.URL.Path` as the storage key, so callers are
responsible for stripping any mount prefix before the handler sees
the request.

## Building URLs in Templates

Whenever Den is opened with `den.WithStorage(...)`, Burrow
auto-registers the `mediaURL` template function globally — no per-app
`FuncMap` required. The function is `den.Storage.URL`, so it composes
the same URL whether the backend is a local `FilesystemStorage`
(relative URL) or a remote backend (absolute URL):

```html
<!-- HAS-file: named Attachment field -->
<img src="{{ mediaURL .Hero }}" alt="{{ .HeroAlt }}">

<!-- IS-a-file: embedded Attachment, reached via its type name -->
<img src="{{ mediaURL .Media.Attachment }}" alt="{{ .Media.AltText }}">
```

The same template line keeps working unchanged if you swap the
Storage for S3 or a CDN later — remote backends return absolute URLs
from the same `URL()` call.

If no Storage is configured, `mediaURL` is not registered and any
`{{ mediaURL ... }}` call fails at template-parse time — the right
signal that the app has an unwired dependency.

## Writing a Custom Storage Backend

Implement `den.Storage` — see
[Den's attachments guide](https://den-odm.readthedocs.io/guide/attachments/#writing-a-custom-storage-backend)
for the full contract. Implement the optional `URLPrefix() string`
method only when the backend serves files through the application's
own HTTP server; remote backends that return absolute URLs should
omit it. `uploader.Mount` skips route registration for backends
without `URLPrefix()`.

Hand the backend to Den — `Uploader` inherits it automatically:

```go
s3 := newS3Storage(cfg)
db, _ := den.OpenURL(ctx, dsn, den.WithStorage(s3))
u := uploader.NewUploader(db) // uses s3
```

## Errors

| Error | Description |
|---|---|
| `uploader.ErrTypeNotAllowed` | File MIME type not in allowed list |
| `uploader.ErrFileTooLarge` | File exceeds `MaxSize` |
| `storage.ErrEmptyContent` | Uploaded file is empty (re-exported via `den/storage`; same sentinel the backend would return) |
| `uploader.ErrMissingField` | Form field not found in request |

## Public API

| Identifier | Description |
|---|---|
| `uploader.NewUploader(*den.DB) *Uploader` | Construct an Uploader. Panics if the DB has no Storage. |
| `(*Uploader).Store(r, field, opts) (document.Attachment, error)` | Validate and persist a multipart file. |
| `(*Uploader).Storage() den.Storage` | Returns the bound Storage — useful for compensating deletes. |
| `uploader.Mount(chi.Router, den.Storage)` | Register a serving handler at the Storage's `URLPrefix()`. No-op for backends without `URLPrefix()`. |
| `uploader.ServeHandler(den.Storage) http.Handler` | Raw serving handler; treats `r.URL.Path` as the storage key. Hand-mount with `http.StripPrefix` or `chi.Mount`. |
