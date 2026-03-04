package uploads

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
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

// Storage defines the interface for file storage backends.
type Storage interface {
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
func StoreFile(r *http.Request, fieldName string, storage Storage, opts StoreOptions) (string, error) {
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

// contentHash returns the first 16 hex characters of the SHA-256 hash.
func contentHash(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}

// buildKey constructs a storage key from prefix, hash, and original filename.
func buildKey(prefix, hash, filename string) string {
	ext := strings.ToLower(path.Ext(filename))
	if ext == "" {
		ext = ".bin"
	}

	name := hash + ext
	if prefix != "" {
		return prefix + "/" + name
	}
	return name
}

// detectMIME reads the first 512 bytes to detect the content type.
// It returns the MIME type and the full content (header + rest).
func detectMIME(file io.Reader) (mimeType string, content []byte, err error) {
	header := make([]byte, 512)
	n, err := io.ReadAtLeast(file, header, 1)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			if n == 0 {
				return "", nil, ErrEmptyFile
			}
		} else {
			return "", nil, fmt.Errorf("read file header: %w", err)
		}
	}
	header = header[:n]

	mimeType = http.DetectContentType(header)

	rest, err := io.ReadAll(file)
	if err != nil {
		return "", nil, fmt.Errorf("read file body: %w", err)
	}

	content = make([]byte, len(header)+len(rest))
	copy(content, header)
	copy(content[len(header):], rest)

	return mimeType, content, nil
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
