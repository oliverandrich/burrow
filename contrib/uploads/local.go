package uploads

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathTraversal is returned when a storage key attempts to escape the root directory.
var ErrPathTraversal = errors.New("uploads: key escapes root directory")

// LocalStorage stores files on the local filesystem.
type LocalStorage struct {
	root      string
	urlPrefix string
}

// NewLocalStorage creates a LocalStorage that persists files under root
// and serves them at urlPrefix.
func NewLocalStorage(root, urlPrefix string) (*LocalStorage, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("uploads: create root dir: %w", err)
	}
	return &LocalStorage{root: root, urlPrefix: urlPrefix}, nil
}

// Store persists a file and returns its content-hashed storage key.
// The file is streamed to a temp file while simultaneously computing
// the SHA-256 hash — only the first 512 bytes are buffered in memory
// for MIME detection.
func (s *LocalStorage) Store(_ context.Context, file io.Reader, opts StoreOptions) (string, error) {
	// 1. Detect MIME type from first 512 bytes.
	mimeType, header, err := detectMIMEType(file)
	if err != nil {
		return "", err
	}

	if !isTypeAllowed(mimeType, opts.AllowedTypes) {
		return "", ErrTypeNotAllowed
	}

	// 2. Create temp file for streaming.
	tmp, err := os.CreateTemp(s.root, ".upload-*")
	if err != nil {
		return "", fmt.Errorf("uploads: create temp file: %w", err)
	}
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name()) //nolint:gosec // tmp.Name() is from os.CreateTemp
	}

	// 3. Recombine header bytes with the remaining stream.
	combined := io.MultiReader(bytes.NewReader(header), file)

	// 4. Stream through SHA-256 hasher to temp file.
	hasher := sha256.New()
	reader := io.TeeReader(combined, hasher)

	var written int64
	if opts.MaxSize > 0 {
		// Read at most MaxSize+1 bytes — if we get more, the file is too large.
		written, err = io.Copy(tmp, io.LimitReader(reader, opts.MaxSize+1))
		if err != nil {
			cleanup()
			return "", fmt.Errorf("uploads: stream to temp file: %w", err)
		}
		if written > opts.MaxSize {
			cleanup()
			return "", ErrFileTooLarge
		}
	} else {
		written, err = io.Copy(tmp, reader)
		if err != nil {
			cleanup()
			return "", fmt.Errorf("uploads: stream to temp file: %w", err)
		}
	}

	if written == 0 {
		cleanup()
		return "", ErrEmptyFile
	}

	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name()) //nolint:gosec // tmp.Name() is from os.CreateTemp
		return "", fmt.Errorf("uploads: close temp file: %w", err)
	}

	// 5. Build storage key from content hash.
	hash := fmt.Sprintf("%x", hasher.Sum(nil)[:8])
	key := buildKey(opts.Prefix, hash, mimeType, opts.Filename)
	dst := filepath.Join(s.root, key)

	// 6. Deduplication: if file already exists, discard temp.
	if _, statErr := os.Stat(dst); statErr == nil { //nolint:gosec // dst is built from root + content-hash
		_ = os.Remove(tmp.Name()) //nolint:gosec // tmp.Name() is from os.CreateTemp
		return key, nil
	}

	// 7. Create parent directories and rename temp → final.
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil { //nolint:gosec // dst is built from root + content-hash
		_ = os.Remove(tmp.Name()) //nolint:gosec // tmp.Name() is from os.CreateTemp
		return "", fmt.Errorf("uploads: create dir: %w", err)
	}
	if err := os.Rename(tmp.Name(), dst); err != nil { //nolint:gosec // tmp.Name() from os.CreateTemp
		_ = os.Remove(tmp.Name()) //nolint:gosec // tmp.Name() is from os.CreateTemp
		return "", fmt.Errorf("uploads: rename temp file: %w", err)
	}

	return key, nil
}

// Delete removes the file at the given key. It does not return an error
// if the file does not exist. Returns [ErrPathTraversal] if the key
// would escape the root directory.
func (s *LocalStorage) Delete(_ context.Context, key string) error {
	path, err := s.safeJoin(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Open returns a reader for the file at the given key.
// Returns [ErrPathTraversal] if the key would escape the root directory.
func (s *LocalStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := s.safeJoin(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path) //nolint:gosec // path is validated by safeJoin
}

// URL returns the public URL for the given storage key.
func (s *LocalStorage) URL(key string) string {
	return s.urlPrefix + key
}

// Path returns the filesystem path for the given storage key.
// Returns [ErrPathTraversal] if the key would escape the root directory.
// This is specific to LocalStorage and not part of the Store interface.
func (s *LocalStorage) Path(key string) (string, error) {
	return s.safeJoin(key)
}

// safeJoin joins the root and key, then verifies the result is still
// under root. This prevents path traversal attacks via keys containing "..".
func (s *LocalStorage) safeJoin(key string) (string, error) {
	path := filepath.Join(s.root, key)
	if !strings.HasPrefix(path, filepath.Clean(s.root)+string(os.PathSeparator)) {
		return "", ErrPathTraversal
	}
	return path, nil
}
