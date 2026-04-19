// Package uploader provides HTTP-layer helpers for file uploads built on
// top of [den.Storage]: an [Uploader] service that validates and persists
// multipart uploads into a [document.Attachment], a [ServeHandler] that
// streams stored bytes back to HTTP clients, and a [Mount] helper that
// registers the serving handler at the Storage's own URL prefix.
//
// The package does not implement a burrow.App. The application owns the
// routing:
//
//	u := uploader.NewUploader(db)
//	uploader.Mount(r, db.Storage())
//	r.Post("/upload", func(w http.ResponseWriter, r *http.Request) {
//	    att, err := u.Store(r, "file", uploader.StoreOptions{
//	        AllowedTypes: []string{"image/"},
//	        MaxSize:      10 << 20,
//	    })
//	    // ...
//	})
//
// All Storage configuration — root directory, URL prefix, backend choice —
// lives on the [den.Storage] passed to [den.WithStorage] at DB open time.
// The URL prefix is written once (at Storage construction) and read by
// both [den.Storage.URL] (for template URL generation, exposed globally
// via the `mediaURL` template function) and [Mount] (for route registration).
package uploader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"
	"github.com/oliverandrich/den/storage"
)

// Sentinel errors for upload validation.
//
// Empty uploads surface as [storage.ErrEmptyContent] — the same sentinel
// a Storage backend returns when a zero-byte reader reaches it. That
// lets handler code check for "empty upload" uniformly whether the
// emptiness was caught at the HTTP layer or inside the backend.
var (
	ErrTypeNotAllowed = errors.New("uploader: file type not allowed")
	ErrFileTooLarge   = errors.New("uploader: file too large")
	ErrMissingField   = errors.New("uploader: missing form field")
)

// Uploader binds a [den.Storage] to the HTTP ingress helpers. Construct
// once per server with [NewUploader] and reuse across handlers.
type Uploader struct {
	storage den.Storage
}

// NewUploader returns an Uploader that stores files into the Storage
// installed on db via [den.WithStorage]. Panics when the db has no
// Storage configured — that state always indicates a setup bug the
// program cannot recover from.
func NewUploader(db *den.DB) *Uploader {
	s := db.Storage()
	if s == nil {
		panic("uploader: NewUploader: db has no Storage — configure den.WithStorage(...) at OpenURL")
	}
	return &Uploader{storage: s}
}

// Storage returns the bound backend. Useful for compensating deletes on
// record-save errors: `_ = u.Storage().Delete(ctx, att)`.
func (u *Uploader) Storage() den.Storage { return u.storage }

// StoreOptions configures a single [Uploader.Store] invocation.
type StoreOptions struct {
	// AllowedTypes restricts uploads to a MIME allow-list. An empty
	// slice allows all types. Matching is prefix-based so entries like
	// "image/" broaden to any image subtype.
	AllowedTypes []string

	// MaxSize is the per-file upper bound in bytes. Zero means no limit.
	// When the limit is exceeded the upload is aborted mid-stream with
	// ErrFileTooLarge — no partial file is persisted.
	MaxSize int64

	// Filename is the original filename, used as a fallback when the
	// detected MIME type has no mapped extension. Populated from the
	// multipart header automatically if left empty.
	Filename string
}

// Store extracts a file from a multipart request, validates it (MIME
// allow-list, max size, non-empty), and persists it through the bound
// Storage. The returned [document.Attachment] is ready to assign onto a
// document field (embedded or named) before the subsequent
// den.Insert / den.Update.
//
// Typical IS-a-file usage (the document IS the file):
//
//	att, err := u.Store(r, "file", opts)
//	if err != nil { return err }
//	media := &Media{Attachment: att, AltText: "..."}
//	if err := den.Insert(r.Context(), db, media); err != nil {
//	    // Insert failed after bytes were stored — clean up to avoid
//	    // an orphan. Best-effort, log if needed.
//	    _ = u.Storage().Delete(r.Context(), att)
//	    return err
//	}
//
// Typical HAS-file usage (the document points at a file):
//
//	att, err := u.Store(r, "hero", opts)
//	if err != nil { return err }
//	post.Hero = att
//	if err := den.Update(r.Context(), db, post); err != nil {
//	    _ = u.Storage().Delete(r.Context(), att)
//	    return err
//	}
func (u *Uploader) Store(r *http.Request, fieldName string, opts StoreOptions) (document.Attachment, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return document.Attachment{}, fmt.Errorf("%w: %s", ErrMissingField, fieldName)
	}
	defer file.Close() //nolint:errcheck // best-effort close on multipart file

	if opts.Filename == "" {
		opts.Filename = header.Filename
	}

	// http.DetectContentType on the first 512 bytes is authoritative —
	// the client's Content-Type header is trivially spoofable.
	mimeType, headerBytes, err := detectMIMEType(file)
	if err != nil {
		return document.Attachment{}, err
	}

	if !isTypeAllowed(mimeType, opts.AllowedTypes) {
		return document.Attachment{}, ErrTypeNotAllowed
	}

	body := io.Reader(io.MultiReader(bytes.NewReader(headerBytes), file))

	// MaxSize via a mid-stream bounded reader: Read returns
	// ErrFileTooLarge on overflow, Storage.Store aborts, nothing lands
	// on disk.
	if opts.MaxSize > 0 {
		body = &maxBytesReader{r: body, max: opts.MaxSize}
	}

	// MIME-derived extension blocks attacker-controlled ones
	// (e.g. "evil.php" uploaded as image/jpeg).
	ext := extFromMIME(mimeType)
	if ext == "" {
		ext = strings.ToLower(path.Ext(opts.Filename))
	}
	if ext == "" {
		ext = ".bin"
	}

	att, err := u.storage.Store(r.Context(), body, ext, mimeType)
	if err != nil {
		if errors.Is(err, ErrFileTooLarge) {
			return document.Attachment{}, ErrFileTooLarge
		}
		return document.Attachment{}, fmt.Errorf("uploader: store: %w", err)
	}
	return att, nil
}

// urlPrefixer is satisfied by Storage backends that serve through the
// application's own HTTP server (return relative URL paths). Remote
// backends (S3, GCS, CDN) return absolute URLs and intentionally do not
// implement this, which signals [Mount] to skip route registration.
type urlPrefixer interface {
	URLPrefix() string
}

// Mount registers a [ServeHandler] on r at the URL prefix the Storage
// advertises via URLPrefix(). Writes the prefix only once at Storage
// construction — routing derives from it. Safe to call with any
// [den.Storage]: backends that don't implement URLPrefix (remote
// backends serving via absolute URLs) are no-ops.
//
//	fs, _ := file.New("data/media", "/media/")
//	uploader.Mount(r, fs) // registers "/media/*" → ServeHandler(fs)
func Mount(r chi.Router, s den.Storage) {
	sv, ok := s.(urlPrefixer)
	if !ok {
		return
	}
	prefix := strings.TrimRight(sv.URLPrefix(), "/")
	r.Mount(prefix, http.StripPrefix(prefix, ServeHandler(s)))
}

// ServeHandler returns an http.Handler that streams bytes from the
// Storage, with aggressive caching (Cache-Control immutable max-age=1y).
// Files are content-addressed so a changed file gets a new URL, which
// makes the long cache safe.
//
// Treats the incoming request's URL path as the storage key directly,
// so callers must strip any mount prefix first. [Mount] does this
// automatically via chi.Router.Mount; hand-routed callers wrap with
// [http.StripPrefix] (or chi's Mount) themselves.
func ServeHandler(s den.Storage) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/")
		if key == "" || strings.HasSuffix(key, "/") {
			http.NotFound(w, r)
			return
		}

		att := document.Attachment{StoragePath: key}
		f, err := s.Open(r.Context(), att)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close() //nolint:errcheck // best-effort close on serve

		if mimeType := mime.TypeByExtension(path.Ext(key)); mimeType != "" {
			w.Header().Set("Content-Type", mimeType)
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = io.Copy(w, f)
	})
}

// maxBytesReader returns ErrFileTooLarge as soon as more than max bytes
// have been read. Unlike io.LimitReader (silent EOF at the limit) this
// raises an explicit error so Storage.Store aborts and cleans up
// instead of persisting a truncated file.
type maxBytesReader struct {
	r   io.Reader
	max int64
	n   int64
}

func (m *maxBytesReader) Read(p []byte) (int, error) {
	if m.n >= m.max {
		return 0, ErrFileTooLarge
	}
	remaining := m.max - m.n
	if int64(len(p)) > remaining+1 {
		p = p[:remaining+1]
	}
	n, err := m.r.Read(p)
	m.n += int64(n)
	if m.n > m.max {
		return n, ErrFileTooLarge
	}
	return n, err
}

// preferredExts maps common MIME types to a preferred extension —
// mime.ExtensionsByType returns alphabetically, which picks obscure
// variants (e.g. ".jfif" for "image/jpeg").
var preferredExts = map[string]string{
	"image/jpeg":       ".jpg",
	"image/png":        ".png",
	"image/gif":        ".gif",
	"image/webp":       ".webp",
	"image/svg+xml":    ".svg",
	"image/avif":       ".avif",
	"application/pdf":  ".pdf",
	"text/plain":       ".txt",
	"text/html":        ".html",
	"text/css":         ".css",
	"text/csv":         ".csv",
	"application/json": ".json",
	"application/xml":  ".xml",
	"application/zip":  ".zip",
}

// extFromMIME returns a safe file extension for the MIME type, or ""
// if no mapping is known.
func extFromMIME(mimeType string) string {
	if i := strings.IndexByte(mimeType, ';'); i >= 0 {
		mimeType = strings.TrimSpace(mimeType[:i])
	}
	if ext, ok := preferredExts[mimeType]; ok {
		return ext
	}
	exts, err := mime.ExtensionsByType(mimeType)
	if err != nil || len(exts) == 0 {
		return ""
	}
	return exts[0]
}

// detectMIMEType reads the first 512 bytes of r and returns the detected
// MIME type plus the header slice for recombination with io.MultiReader.
func detectMIMEType(r io.Reader) (mimeType string, header []byte, err error) {
	buf := make([]byte, 512)
	n, err := io.ReadAtLeast(r, buf, 1)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			if n == 0 {
				return "", nil, storage.ErrEmptyContent
			}
		} else {
			return "", nil, fmt.Errorf("uploader: read file header: %w", err)
		}
	}
	header = buf[:n]
	mimeType = http.DetectContentType(header)
	return mimeType, header, nil
}

// isTypeAllowed reports whether mimeType matches any entry in allowed.
// Empty list allows all. Matching is prefix-based: "image/" broadens to
// any image subtype; "text/plain" still matches "text/plain; charset=utf-8".
func isTypeAllowed(mimeType string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if strings.HasPrefix(mimeType, a) {
			return true
		}
	}
	return false
}
