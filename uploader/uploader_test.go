package uploader_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow/uploader"
	"github.com/oliverandrich/den"
	_ "github.com/oliverandrich/den/backend/sqlite" // register sqlite:// scheme
	"github.com/oliverandrich/den/document"
	"github.com/oliverandrich/den/storage/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Fixtures ---

func newTestStorage(t *testing.T) *file.Storage {
	t.Helper()
	s, err := file.New(t.TempDir(), "/media")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func newTestDBWithStorage(t *testing.T) (*den.DB, *file.Storage) {
	t.Helper()
	fs := newTestStorage(t)
	db, err := den.OpenURL(t.Context(), "sqlite://:memory:", den.WithStorage(fs))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, fs
}

func newTestDBNoStorage(t *testing.T) *den.DB {
	t.Helper()
	db, err := den.OpenURL(t.Context(), "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func postMultipart(t *testing.T, field, filename string, body []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, err = part.Write(body)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

// --- NewUploader ---

func TestNewUploader_Succeeds(t *testing.T) {
	db, fs := newTestDBWithStorage(t)
	u := uploader.NewUploader(db)
	assert.Same(t, den.Storage(fs), u.Storage())
}

func TestNewUploader_PanicsWithoutStorage(t *testing.T) {
	db := newTestDBNoStorage(t)
	assert.PanicsWithValue(
		t,
		"uploader: NewUploader: db has no Storage — configure den.WithStorage(...) at OpenURL",
		func() { uploader.NewUploader(db) },
	)
}

// --- Uploader.Store ---

func TestStore_MissingField(t *testing.T) {
	db, _ := newTestDBWithStorage(t)
	u := uploader.NewUploader(db)

	req := postMultipart(t, "other", "x.txt", []byte("hi"))
	require.NoError(t, req.ParseMultipartForm(32<<20))

	_, err := u.Store(req, "file", uploader.StoreOptions{})
	require.ErrorIs(t, err, uploader.ErrMissingField)
}

func TestStore_ReturnsPopulatedAttachment(t *testing.T) {
	db, _ := newTestDBWithStorage(t)
	u := uploader.NewUploader(db)

	content := []byte("the quick brown fox jumps over the lazy dog")
	req := postMultipart(t, "file", "fox.txt", content)
	require.NoError(t, req.ParseMultipartForm(32<<20))

	att, err := u.Store(req, "file", uploader.StoreOptions{})
	require.NoError(t, err)

	assert.NotEmpty(t, att.StoragePath)
	assert.Contains(t, att.Mime, "text/plain")
	assert.Equal(t, int64(len(content)), att.Size)
	assert.Len(t, att.SHA256, 64)
}

func TestStore_AllowListRejectsDisallowedType(t *testing.T) {
	db, _ := newTestDBWithStorage(t)
	u := uploader.NewUploader(db)

	req := postMultipart(t, "file", "note.txt", []byte("plain text"))
	require.NoError(t, req.ParseMultipartForm(32<<20))

	_, err := u.Store(req, "file", uploader.StoreOptions{
		AllowedTypes: []string{"image/"},
	})
	require.ErrorIs(t, err, uploader.ErrTypeNotAllowed)
}

func TestStore_AllowListAcceptsMatchingType(t *testing.T) {
	db, _ := newTestDBWithStorage(t)
	u := uploader.NewUploader(db)

	buf := make([]byte, 0, 15)
	buf = append(buf, 0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A)
	buf = append(buf, []byte("padding")...)
	req := postMultipart(t, "file", "logo.png", buf)
	require.NoError(t, req.ParseMultipartForm(32<<20))

	att, err := u.Store(req, "file", uploader.StoreOptions{
		AllowedTypes: []string{"image/png"},
	})
	require.NoError(t, err)
	assert.Equal(t, "image/png", att.Mime)
}

func TestStore_MaxSizeExceeded(t *testing.T) {
	db, _ := newTestDBWithStorage(t)
	u := uploader.NewUploader(db)

	payload := bytes.Repeat([]byte("A"), 2048)
	req := postMultipart(t, "file", "big.txt", payload)
	require.NoError(t, req.ParseMultipartForm(32<<20))

	_, err := u.Store(req, "file", uploader.StoreOptions{MaxSize: 1024})
	require.ErrorIs(t, err, uploader.ErrFileTooLarge)
}

func TestStore_MaxSizeUnderLimit(t *testing.T) {
	db, _ := newTestDBWithStorage(t)
	u := uploader.NewUploader(db)

	payload := bytes.Repeat([]byte("A"), 500)
	req := postMultipart(t, "file", "small.txt", payload)
	require.NoError(t, req.ParseMultipartForm(32<<20))

	att, err := u.Store(req, "file", uploader.StoreOptions{MaxSize: 1024})
	require.NoError(t, err)
	assert.Equal(t, int64(500), att.Size)
}

// --- Mount ---

func TestMount_RegistersAtStoragePrefix(t *testing.T) {
	s := newTestStorage(t) // URLPrefix "/media"
	ctx := context.Background()

	att, err := s.Store(ctx, bytes.NewReader([]byte("served content")), ".txt", "text/plain")
	require.NoError(t, err)

	r := chi.NewRouter()
	uploader.Mount(r, s)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/media/"+att.StoragePath, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/plain")
	assert.Contains(t, rec.Body.String(), "served content")
}

func TestMount_NotFound(t *testing.T) {
	s := newTestStorage(t)

	r := chi.NewRouter()
	uploader.Mount(r, s)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/media/no/such/file.jpg", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	// Regression: a 404 must not carry the year-long immutable Cache-Control,
	// which would poison browser/CDN caches for the missing path.
	assert.Empty(t, rec.Header().Get("Cache-Control"), "404 must not be cached")
}

// remoteOnlyStorage satisfies den.Storage but not urlPrefixer — Mount
// should be a no-op; ServeHandler can still be hand-mounted with
// http.StripPrefix.
type remoteOnlyStorage struct {
	opens map[string][]byte
}

func (r *remoteOnlyStorage) Store(_ context.Context, _ io.Reader, _, _ string) (document.Attachment, error) {
	return document.Attachment{}, nil
}

func (r *remoteOnlyStorage) Open(_ context.Context, a document.Attachment) (io.ReadCloser, error) {
	data, ok := r.opens[a.StoragePath]
	if !ok {
		return nil, errNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (r *remoteOnlyStorage) Delete(_ context.Context, _ document.Attachment) error { return nil }
func (r *remoteOnlyStorage) URL(a document.Attachment) string {
	return "https://cdn.example.com/" + a.StoragePath
}

var errNotFound = errors.New("not found")

func TestMount_NoopForStorageWithoutURLPrefix(t *testing.T) {
	s := &remoteOnlyStorage{opens: map[string][]byte{"foo.txt": []byte("body")}}

	r := chi.NewRouter()
	uploader.Mount(r, s) // no URLPrefix → no route

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/foo.txt", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- ServeHandler (direct, for hand-mounted routing) ---

func TestServeHandler_HandMounted(t *testing.T) {
	s := &remoteOnlyStorage{opens: map[string][]byte{"foo.txt": []byte("body")}}

	r := chi.NewRouter()
	r.Handle("/assets/*", http.StripPrefix("/assets/", uploader.ServeHandler(s)))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/assets/foo.txt", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "body", rec.Body.String())
}

// TestServeHandler_Range exercises the SeekableStorage fast path: the
// file backend implements den.SeekableStorage, so http.ServeContent is
// used and a Range request returns 206 Partial Content with the right
// byte slice.
func TestServeHandler_Range(t *testing.T) {
	fs := newTestStorage(t)

	body := []byte("the quick brown fox jumps over the lazy dog")
	att, err := fs.Store(t.Context(), bytes.NewReader(body), ".txt", "text/plain")
	require.NoError(t, err)

	r := chi.NewRouter()
	uploader.Mount(r, fs)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/media/"+att.StoragePath, nil)
	req.Header.Set("Range", "bytes=20-24")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusPartialContent, rec.Code, "ServeContent should respond with 206 for a valid Range")
	assert.Equal(t, "jumps", rec.Body.String(), "Range bytes=20-24 should be 'jumps'")
	assert.NotEmpty(t, rec.Header().Get("ETag"), "ETag must be present on the response")
}

// TestServeHandler_IfNoneMatch exercises conditional GET: a client that
// already has the bytes (matching ETag) gets 304 Not Modified with no
// body re-transmission.
func TestServeHandler_IfNoneMatch(t *testing.T) {
	fs := newTestStorage(t)

	body := []byte("immutable bytes")
	att, err := fs.Store(t.Context(), bytes.NewReader(body), ".txt", "text/plain")
	require.NoError(t, err)

	r := chi.NewRouter()
	uploader.Mount(r, fs)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/media/"+att.StoragePath, nil)
	req.Header.Set("If-None-Match", `"`+att.StoragePath+`"`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotModified, rec.Code, "matching If-None-Match should return 304")
	assert.Empty(t, rec.Body.String(), "304 must not re-send the body")
}

// --- End-to-end ---

func TestEndToEnd_UploadThenServe(t *testing.T) {
	db, fs := newTestDBWithStorage(t)
	u := uploader.NewUploader(db)

	r := chi.NewRouter()
	uploader.Mount(r, fs)
	r.Post("/upload", func(w http.ResponseWriter, r *http.Request) {
		att, err := u.Store(r, "file", uploader.StoreOptions{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(att.StoragePath))
	})

	payload := []byte("document content")
	uploadRec := httptest.NewRecorder()
	r.ServeHTTP(uploadRec, postMultipart(t, "file", "doc.txt", payload))

	require.Equal(t, http.StatusCreated, uploadRec.Code)
	storagePath := uploadRec.Body.String()
	assert.NotEmpty(t, storagePath)

	getReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/media/"+storagePath, nil)
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)

	assert.Equal(t, http.StatusOK, getRec.Code)
	got, err := io.ReadAll(getRec.Body)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}
