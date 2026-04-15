package uploads

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
)

// Sentinel errors for upload validation.
var (
	ErrTypeNotAllowed = errors.New("uploads: file type not allowed")
	ErrFileTooLarge   = errors.New("uploads: file too large")
	ErrEmptyFile      = errors.New("uploads: empty file")
	ErrMissingField   = errors.New("uploads: missing form field")
)

// Store defines the interface for file storage backends.
type Store interface {
	// Store persists a file and returns its storage key.
	Store(ctx context.Context, file io.Reader, opts StoreOptions) (key string, err error)

	// Delete removes a file by its storage key.
	Delete(ctx context.Context, key string) error

	// Open returns a reader for the file at the given key.
	Open(ctx context.Context, key string) (io.ReadCloser, error)

	// URL returns the public URL for the given storage key.
	URL(key string) string
}

// StoreOptions configures a single file store operation.
type StoreOptions struct {
	Prefix       string   // subdirectory, e.g. "avatars"
	Filename     string   // original filename (for extension extraction)
	AllowedTypes []string // e.g. ["image/jpeg", "image/png"]; empty = all
	MaxSize      int64    // per-file limit in bytes (0 = no limit)
}

// StoreFile extracts a file from a multipart request and stores it.
// It returns the storage key on success.
func StoreFile(r *http.Request, fieldName string, storage Store, opts StoreOptions) (string, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrMissingField, fieldName)
	}
	defer file.Close() //nolint:errcheck // best-effort close on multipart file

	if opts.Filename == "" {
		opts.Filename = header.Filename
	}

	// Apply default allowed types from context if not set per-call.
	if len(opts.AllowedTypes) == 0 {
		if defaults := allowedTypesFromContext(r.Context()); len(defaults) > 0 {
			opts.AllowedTypes = defaults
		}
	}

	return storage.Store(r.Context(), file, opts)
}

// buildKey constructs a storage key from prefix, hash, and MIME type.
// The extension is derived from the detected MIME type rather than the
// original filename to prevent attacker-controlled extensions (e.g.,
// uploading a JPEG with filename "evil.php").
func buildKey(prefix, hash, mimeType, filename string) string {
	ext := extFromMIME(mimeType)
	if ext == "" {
		// Fall back to the original filename extension if the MIME type
		// has no known mapping.
		ext = strings.ToLower(path.Ext(filename))
	}
	if ext == "" {
		ext = ".bin"
	}

	name := hash + ext
	if prefix != "" {
		return prefix + "/" + name
	}
	return name
}

// preferredExts maps common MIME types to their preferred extension.
// mime.ExtensionsByType returns extensions in alphabetical order, which
// can pick obscure extensions (e.g., ".jfif" for "image/jpeg").
var preferredExts = map[string]string{
	"image/jpeg":       ".jpg",
	"image/png":        ".png",
	"image/gif":        ".gif",
	"image/webp":       ".webp",
	"image/svg+xml":    ".svg",
	"application/pdf":  ".pdf",
	"text/plain":       ".txt",
	"text/html":        ".html",
	"text/css":         ".css",
	"text/csv":         ".csv",
	"application/json": ".json",
	"application/xml":  ".xml",
	"application/zip":  ".zip",
}

// extFromMIME returns a safe file extension for the given MIME type.
// Returns "" if the MIME type has no known extension.
func extFromMIME(mimeType string) string {
	// Strip parameters (e.g., "text/plain; charset=utf-8" → "text/plain").
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

// detectMIMEType reads the first 512 bytes to detect the content type.
// It returns the MIME type and the bytes read so far. The caller is
// responsible for recombining the header with the remaining stream
// (e.g., via io.MultiReader).
func detectMIMEType(file io.Reader) (mimeType string, header []byte, err error) {
	buf := make([]byte, 512)
	n, err := io.ReadAtLeast(file, buf, 1)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			if n == 0 {
				return "", nil, ErrEmptyFile
			}
		} else {
			return "", nil, fmt.Errorf("read file header: %w", err)
		}
	}
	header = buf[:n]
	mimeType = http.DetectContentType(header)
	return mimeType, header, nil
}

// isTypeAllowed checks if the MIME type is in the allowed list.
// An empty list allows all types. Matching is prefix-based to handle
// parameters (e.g. "text/plain; charset=utf-8" matches "text/plain").
func isTypeAllowed(mimeType string, allowedTypes []string) bool {
	if len(allowedTypes) == 0 {
		return true
	}
	for _, allowed := range allowedTypes {
		if strings.HasPrefix(mimeType, allowed) {
			return true
		}
	}
	return false
}
