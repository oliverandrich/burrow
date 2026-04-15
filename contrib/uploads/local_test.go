package uploads

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helper function tests ---

func TestBuildKey(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		hash     string
		mimeType string
		filename string
		want     string
	}{
		{"ext from mime", "avatars", "abc123", "image/jpeg", "photo.JPG", "avatars/abc123.jpg"},
		{"no prefix", "", "abc123", "image/png", "photo.png", "abc123.png"},
		{"unknown mime falls back to filename", "", "abc123", "application/x-unknown", "data.csv", "abc123.csv"},
		{"no mime no ext", "", "abc123", "", "noext", "abc123.bin"},
		{"mime prevents malicious ext", "uploads", "abc123", "image/jpeg", "evil.php", "uploads/abc123.jpg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildKey(tt.prefix, tt.hash, tt.mimeType, tt.filename)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectMIMEType(t *testing.T) {
	t.Run("detects PNG", func(t *testing.T) {
		png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		mime, header, err := detectMIMEType(bytes.NewReader(png))
		require.NoError(t, err)
		assert.Equal(t, "image/png", mime)
		assert.Equal(t, png, header)
	})

	t.Run("detects plain text", func(t *testing.T) {
		text := []byte("Hello, World!")
		mime, header, err := detectMIMEType(bytes.NewReader(text))
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(mime, "text/plain"))
		assert.Equal(t, text, header)
	})

	t.Run("empty file", func(t *testing.T) {
		_, _, err := detectMIMEType(bytes.NewReader(nil))
		assert.ErrorIs(t, err, ErrEmptyFile)
	})

	t.Run("returns only header not full content", func(t *testing.T) {
		data := bytes.Repeat([]byte("x"), 2048)
		_, header, err := detectMIMEType(bytes.NewReader(data))
		require.NoError(t, err)
		assert.Len(t, header, 512, "should return at most 512 bytes")
	})

	t.Run("reader error propagates", func(t *testing.T) {
		_, _, err := detectMIMEType(&failingReader{err: io.ErrClosedPipe})
		require.Error(t, err)
		assert.ErrorIs(t, err, io.ErrClosedPipe)
	})
}

func TestIsTypeAllowed(t *testing.T) {
	assert.True(t, isTypeAllowed("image/png", nil), "empty list allows all")
	assert.True(t, isTypeAllowed("image/png", []string{"image/png", "image/jpeg"}))
	assert.False(t, isTypeAllowed("text/plain", []string{"image/png", "image/jpeg"}))
	assert.True(t, isTypeAllowed("text/plain; charset=utf-8", []string{"text/plain"}), "prefix match")
}

// failingReader always returns an error on Read.
type failingReader struct {
	err error
}

func (r *failingReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

// --- LocalStorage tests ---

func TestNewLocalStorage_InvalidRoot(t *testing.T) {
	// Use a path under a file (not a directory) to trigger MkdirAll failure
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "afile")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0o644))

	_, err := NewLocalStorage(filepath.Join(filePath, "subdir"), "/media/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create root dir")
}

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
	p, err := s.Path("avatars/abc.jpg")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(s.root, "avatars/abc.jpg"), p)
}

func TestLocalStorage_OpenRejectsPathTraversal(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	traversalKeys := []string{
		"../../etc/passwd",
		"../secret.txt",
		"subdir/../../etc/shadow",
	}

	for _, key := range traversalKeys {
		_, err := s.Open(ctx, key)
		require.Error(t, err, "Open should reject traversal key: %s", key)
		assert.ErrorIs(t, err, ErrPathTraversal, "Open should return ErrPathTraversal for: %s", key)
	}
}

func TestLocalStorage_DeleteRejectsPathTraversal(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	err := s.Delete(ctx, "../../etc/passwd")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPathTraversal)
}

func TestLocalStorage_PathRejectsPathTraversal(t *testing.T) {
	s := newTestStorage(t)

	_, err := s.Path("../../etc/passwd")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPathTraversal)
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

	// "data" is detected as text/plain → extension derived from MIME type, not filename.
	key, err := s.Store(ctx, bytes.NewReader([]byte("data")), StoreOptions{Filename: "noext"})
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(key, ".txt"), "text content should get .txt extension from MIME detection")
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
	diskPath, pathErr := s.Path(key)
	require.NoError(t, pathErr)
	_, err = os.Stat(diskPath)
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

	// Minimal JPEG: SOI marker + padding (detected as image/jpeg by http.DetectContentType).
	content := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0x00}, 20)...)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	key, err := StoreFile(req, "avatar", s, StoreOptions{Prefix: "avatars"})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(key, "avatars/"))
	assert.True(t, strings.HasSuffix(key, ".jpg"), "JPEG content should get .jpg extension from MIME detection, got: %s", key)

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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, err := StoreFile(req, "avatar", s, StoreOptions{})
	assert.ErrorIs(t, err, ErrMissingField)
}

// --- Integration tests ---

func TestLocalStorage_LargeFile(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	// Generate a 2MB file with random data.
	const size = 2 * 1024 * 1024
	data := make([]byte, size)
	_, err := rand.Read(data)
	require.NoError(t, err)

	key, err := s.Store(ctx, bytes.NewReader(data), StoreOptions{
		Prefix:   "large",
		Filename: "bigfile.bin",
	})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(key, "large/"))
	assert.True(t, strings.HasSuffix(key, ".bin"))

	// Verify the file exists on disk with the correct size.
	diskPath, pathErr := s.Path(key)
	require.NoError(t, pathErr)
	info, err := os.Stat(diskPath)
	require.NoError(t, err)
	assert.Equal(t, int64(size), info.Size())

	// Open and verify full content matches.
	rc, err := s.Open(ctx, key)
	require.NoError(t, err)
	defer rc.Close()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestLocalStorage_ConcurrentUploads(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	const numFiles = 20

	// Pre-generate unique file contents so each gets a distinct key.
	contents := make([][]byte, numFiles)
	for i := range contents {
		buf := make([]byte, 1024)
		_, err := rand.Read(buf)
		require.NoError(t, err)
		contents[i] = buf
	}

	keys := make([]string, numFiles)
	errs := make([]error, numFiles)

	var wg sync.WaitGroup
	wg.Add(numFiles)

	for i := range numFiles {
		go func(idx int) {
			defer wg.Done()
			keys[idx], errs[idx] = s.Store(ctx, bytes.NewReader(contents[idx]), StoreOptions{
				Prefix:   "concurrent",
				Filename: "file.dat",
			})
		}(i)
	}
	wg.Wait()

	// Verify all uploads succeeded and each file is retrievable.
	storedKeys := make(map[string]bool)
	for i := range numFiles {
		require.NoError(t, errs[i], "upload %d failed", i)
		storedKeys[keys[i]] = true

		rc, err := s.Open(ctx, keys[i])
		require.NoError(t, err, "open %d failed", i)

		got, err := io.ReadAll(rc)
		require.NoError(t, err)
		rc.Close()

		assert.Equal(t, contents[i], got, "content mismatch for upload %d", i)
	}

	// All files should have unique keys (unique content → unique hash).
	assert.Len(t, storedKeys, numFiles, "expected %d unique keys", numFiles)
}

func TestLocalStorage_MaxSizeBoundary(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	const maxSize int64 = 1024

	t.Run("exactly at limit succeeds", func(t *testing.T) {
		data := bytes.Repeat([]byte("a"), int(maxSize))
		key, err := s.Store(ctx, bytes.NewReader(data), StoreOptions{
			Filename: "exact.txt",
			MaxSize:  maxSize,
		})
		require.NoError(t, err)

		rc, err := s.Open(ctx, key)
		require.NoError(t, err)
		defer rc.Close()

		got, err := io.ReadAll(rc)
		require.NoError(t, err)
		assert.Equal(t, data, got)
	})

	t.Run("one byte over limit fails", func(t *testing.T) {
		data := bytes.Repeat([]byte("a"), int(maxSize)+1)
		_, err := s.Store(ctx, bytes.NewReader(data), StoreOptions{
			Filename: "over.txt",
			MaxSize:  maxSize,
		})
		assert.ErrorIs(t, err, ErrFileTooLarge)
	})

	t.Run("one byte under limit succeeds", func(t *testing.T) {
		data := bytes.Repeat([]byte("a"), int(maxSize)-1)
		key, err := s.Store(ctx, bytes.NewReader(data), StoreOptions{
			Filename: "under.txt",
			MaxSize:  maxSize,
		})
		require.NoError(t, err)

		rc, err := s.Open(ctx, key)
		require.NoError(t, err)
		defer rc.Close()

		got, err := io.ReadAll(rc)
		require.NoError(t, err)
		assert.Equal(t, data, got)
	})
}

func TestLocalStorage_AbortingReader(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	// Reader that delivers some bytes then errors mid-stream.
	r := &abortingReader{
		data: bytes.Repeat([]byte("x"), 2048),
		err:  io.ErrUnexpectedEOF,
		at:   1024,
	}

	_, err := s.Store(ctx, r, StoreOptions{Filename: "abort.txt"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream to temp file")

	// Verify no temp files leaked in storage root.
	entries, dirErr := os.ReadDir(s.root)
	require.NoError(t, dirErr)
	for _, e := range entries {
		assert.False(t, strings.HasPrefix(e.Name(), ".upload-"),
			"temp file %s should have been cleaned up", e.Name())
	}
}

// abortingReader delivers data normally until position 'at', then returns err.
type abortingReader struct {
	data []byte
	err  error
	at   int
	pos  int
}

func (r *abortingReader) Read(p []byte) (int, error) {
	if r.pos >= r.at {
		return 0, r.err
	}
	n := min(len(p), r.at-r.pos)
	copy(p, r.data[r.pos:r.pos+n])
	r.pos += n
	return n, nil
}
