package uploads

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

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
func (s *LocalStorage) Store(_ context.Context, file io.Reader, opts StoreOptions) (string, error) {
	mimeType, content, err := detectMIME(file)
	if err != nil {
		return "", err
	}

	if !isTypeAllowed(mimeType, opts.AllowedTypes) {
		return "", ErrTypeNotAllowed
	}

	if opts.MaxSize > 0 && int64(len(content)) > opts.MaxSize {
		return "", ErrFileTooLarge
	}

	hash := contentHash(content)
	key := buildKey(opts.Prefix, hash, opts.Filename)
	dst := filepath.Join(s.root, key)

	// Deduplication: skip write if file already exists.
	if _, statErr := os.Stat(dst); statErr == nil { //nolint:gosec // G703: dst is built from root + content-hash + sanitized filename
		return key, nil
	}

	if writeErr := atomicWrite(dst, content); writeErr != nil {
		return "", writeErr
	}

	return key, nil
}

// atomicWrite creates parent directories and writes content to dst via
// a temp file + rename for crash safety.
func atomicWrite(dst string, content []byte) error {
	if mkdirErr := os.MkdirAll(filepath.Dir(dst), 0o750); mkdirErr != nil { //nolint:gosec // G703: dst is built from root + content-hash + sanitized filename
		return fmt.Errorf("uploads: create dir: %w", mkdirErr)
	}

	tmp, tmpErr := os.CreateTemp(filepath.Dir(dst), ".upload-*")
	if tmpErr != nil {
		return fmt.Errorf("uploads: create temp file: %w", tmpErr)
	}

	// cleanupTemp removes the temp file; safe to call multiple times.
	cleanupTemp := func() { _ = os.Remove(tmp.Name()) } //nolint:gosec // tmp.Name() is from os.CreateTemp

	if _, writeErr := tmp.Write(content); writeErr != nil {
		_ = tmp.Close()
		cleanupTemp()
		return fmt.Errorf("uploads: write temp file: %w", writeErr)
	}
	if closeErr := tmp.Close(); closeErr != nil {
		cleanupTemp()
		return fmt.Errorf("uploads: close temp file: %w", closeErr)
	}

	if renameErr := os.Rename(tmp.Name(), dst); renameErr != nil { //nolint:gosec // tmp.Name() is from os.CreateTemp
		cleanupTemp()
		return fmt.Errorf("uploads: rename temp file: %w", renameErr)
	}

	return nil
}

// Delete removes the file at the given key. It does not return an error
// if the file does not exist.
func (s *LocalStorage) Delete(_ context.Context, key string) error {
	err := os.Remove(filepath.Join(s.root, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Open returns a reader for the file at the given key.
func (s *LocalStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.root, key)) //nolint:gosec // key is a storage key, not user input
}

// URL returns the public URL for the given storage key.
func (s *LocalStorage) URL(key string) string {
	return s.urlPrefix + key
}

// Path returns the filesystem path for the given storage key.
// This is specific to LocalStorage and not part of the Storage interface.
func (s *LocalStorage) Path(key string) string {
	return filepath.Join(s.root, key)
}
