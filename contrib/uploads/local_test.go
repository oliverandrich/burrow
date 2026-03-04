package uploads

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helper function tests ---

func TestContentHash(t *testing.T) {
	hash := contentHash([]byte("hello world"))
	assert.Len(t, hash, 16)
	assert.Equal(t, hash, contentHash([]byte("hello world")), "same input produces same hash")
	assert.NotEqual(t, hash, contentHash([]byte("different")), "different input produces different hash")
}

func TestBuildKey(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		hash     string
		filename string
		want     string
	}{
		{"with prefix", "avatars", "abc123", "photo.JPG", "avatars/abc123.jpg"},
		{"no prefix", "", "abc123", "photo.png", "abc123.png"},
		{"no extension", "", "abc123", "noext", "abc123.bin"},
		{"uppercase ext", "docs", "abc123", "file.PDF", "docs/abc123.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildKey(tt.prefix, tt.hash, tt.filename)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectMIME(t *testing.T) {
	t.Run("detects PNG", func(t *testing.T) {
		// Minimal PNG header
		png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		mime, content, err := detectMIME(bytes.NewReader(png))
		require.NoError(t, err)
		assert.Equal(t, "image/png", mime)
		assert.Equal(t, png, content)
	})

	t.Run("detects plain text", func(t *testing.T) {
		text := []byte("Hello, World!")
		mime, content, err := detectMIME(bytes.NewReader(text))
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(mime, "text/plain"))
		assert.Equal(t, text, content)
	})

	t.Run("empty file", func(t *testing.T) {
		_, _, err := detectMIME(bytes.NewReader(nil))
		assert.ErrorIs(t, err, ErrEmptyFile)
	})
}

func TestIsTypeAllowed(t *testing.T) {
	assert.True(t, isTypeAllowed("image/png", nil), "empty list allows all")
	assert.True(t, isTypeAllowed("image/png", []string{"image/png", "image/jpeg"}))
	assert.False(t, isTypeAllowed("text/plain", []string{"image/png", "image/jpeg"}))
	assert.True(t, isTypeAllowed("text/plain; charset=utf-8", []string{"text/plain"}), "prefix match")
}

// --- LocalStorage tests ---

func newTestStorage(t *testing.T) *LocalStorage {
	t.Helper()
	dir := t.TempDir()
	s, err := NewLocalStorage(dir, "/media/")
	require.NoError(t, err)
	return s
}

func TestLocalStorage_StoreAndOpen(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	content := []byte("file content for hashing test")
	key, err := s.Store(ctx, bytes.NewReader(content), StoreOptions{
		Prefix:   "docs",
		Filename: "readme.txt",
	})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(key, "docs/"))
	assert.True(t, strings.HasSuffix(key, ".txt"))

	// Open and verify content
	rc, err := s.Open(ctx, key)
	require.NoError(t, err)
	defer rc.Close()
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestLocalStorage_Delete(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	key, err := s.Store(ctx, bytes.NewReader([]byte("to delete")), StoreOptions{Filename: "f.txt"})
	require.NoError(t, err)

	err = s.Delete(ctx, key)
	require.NoError(t, err)

	// File should be gone
	_, err = s.Open(ctx, key)
	require.Error(t, err)

	// Deleting again should not error
	err = s.Delete(ctx, key)
	assert.NoError(t, err)
}

func TestLocalStorage_URL(t *testing.T) {
	s := newTestStorage(t)
	assert.Equal(t, "/media/avatars/abc.jpg", s.URL("avatars/abc.jpg"))
}

func TestLocalStorage_Path(t *testing.T) {
	s := newTestStorage(t)
	p := s.Path("avatars/abc.jpg")
	assert.Equal(t, filepath.Join(s.root, "avatars/abc.jpg"), p)
}

func TestLocalStorage_Deduplication(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	content := []byte("duplicate content")
	key1, err := s.Store(ctx, bytes.NewReader(content), StoreOptions{Filename: "a.txt"})
	require.NoError(t, err)
	key2, err := s.Store(ctx, bytes.NewReader(content), StoreOptions{Filename: "b.txt"})
	require.NoError(t, err)

	// Same content, same extension → same key (dedup)
	assert.Equal(t, key1, key2)
}

func TestLocalStorage_TypeValidation(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	// Plain text content, but only images allowed
	_, err := s.Store(ctx, bytes.NewReader([]byte("not an image")), StoreOptions{
		Filename:     "test.txt",
		AllowedTypes: []string{"image/png", "image/jpeg"},
	})
	assert.ErrorIs(t, err, ErrTypeNotAllowed)
}

func TestLocalStorage_SizeValidation(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	_, err := s.Store(ctx, bytes.NewReader([]byte("too large")), StoreOptions{
		Filename: "big.txt",
		MaxSize:  5,
	})
	assert.ErrorIs(t, err, ErrFileTooLarge)
}

func TestLocalStorage_EmptyFile(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	_, err := s.Store(ctx, bytes.NewReader(nil), StoreOptions{Filename: "empty.txt"})
	assert.ErrorIs(t, err, ErrEmptyFile)
}

func TestLocalStorage_NoExtension(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	key, err := s.Store(ctx, bytes.NewReader([]byte("data")), StoreOptions{Filename: "noext"})
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(key, ".bin"))
}

func TestLocalStorage_PrefixSubdirectory(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	key, err := s.Store(ctx, bytes.NewReader([]byte("nested")), StoreOptions{
		Prefix:   "deep/nested",
		Filename: "file.txt",
	})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(key, "deep/nested/"))

	// File should exist on disk
	_, err = os.Stat(s.Path(key))
	assert.NoError(t, err)
}

// --- StoreFile test ---

func TestStoreFile(t *testing.T) {
	s := newTestStorage(t)

	// Build multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("avatar", "photo.jpg")
	require.NoError(t, err)

	content := []byte("fake image content")
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	key, err := StoreFile(req, "avatar", s, StoreOptions{Prefix: "avatars"})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(key, "avatars/"))
	assert.True(t, strings.HasSuffix(key, ".jpg"))

	// Verify stored content
	rc, err := s.Open(context.Background(), key)
	require.NoError(t, err)
	defer rc.Close()
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestStoreFile_MissingField(t *testing.T) {
	s := newTestStorage(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, err := StoreFile(req, "avatar", s, StoreOptions{})
	assert.ErrorIs(t, err, ErrMissingField)
}
