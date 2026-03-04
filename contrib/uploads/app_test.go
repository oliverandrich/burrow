package uploads

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextHelpers(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	t.Run("nil storage returns key", func(t *testing.T) {
		assert.Equal(t, "some/key.jpg", URL(ctx, "some/key.jpg"))
	})

	t.Run("storage from context", func(t *testing.T) {
		ctx := WithStorage(ctx, s) //nolint:govet // intentional shadow for test clarity
		got := StorageFromContext(ctx)
		require.NotNil(t, got)
		assert.Equal(t, "/media/some/key.jpg", URL(ctx, "some/key.jpg"))
	})
}

func TestMiddlewareInjectsStorage(t *testing.T) {
	s := newTestStorage(t)

	app := &App{storage: s, urlPrefix: "/media/", dir: s.root}
	mw := app.Middleware()
	require.Len(t, mw, 1)

	var gotStorage Storage
	handler := mw[0](http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotStorage = StorageFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.NotNil(t, gotStorage)
}

func TestServingRoute(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	// Store a file first
	key, err := s.Store(ctx, bytes.NewReader([]byte("served content")), StoreOptions{
		Filename: "test.txt",
	})
	require.NoError(t, err)

	app := &App{storage: s, urlPrefix: "/media/", dir: s.root}

	r := chi.NewRouter()
	app.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/media/"+key, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
	assert.Contains(t, rec.Body.String(), "served content")
}

func TestNewWithAllowedTypes(t *testing.T) {
	app := New("./data", "/media/", WithAllowedTypes("image/jpeg", "image/png"))
	assert.Equal(t, []string{"image/jpeg", "image/png"}, app.AllowedTypes())
}

func TestDefaultAllowedTypesApplied(t *testing.T) {
	s := newTestStorage(t)
	app := &App{storage: s, urlPrefix: "/media/", dir: s.root, allowedTypes: []string{"image/png"}}

	r := chi.NewRouter()
	r.Use(app.Middleware()...)

	r.Post("/upload", func(w http.ResponseWriter, r *http.Request) {
		storage := StorageFromContext(r.Context())
		// No AllowedTypes in opts → should use app defaults
		key, err := StoreFile(r, "file", storage, StoreOptions{Prefix: "uploads"})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(key))
	})

	// Upload plain text — should be rejected by app-level allowed types
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "doc.txt")
	require.NoError(t, err)
	_, err = part.Write([]byte("not an image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "not allowed")
}

func TestPerCallAllowedTypesOverrideDefault(t *testing.T) {
	s := newTestStorage(t)
	// App only allows image/png
	app := &App{storage: s, urlPrefix: "/media/", dir: s.root, allowedTypes: []string{"image/png"}}

	r := chi.NewRouter()
	r.Use(app.Middleware()...)

	r.Post("/upload", func(w http.ResponseWriter, r *http.Request) {
		storage := StorageFromContext(r.Context())
		// Per-call opts explicitly allow text/plain → should override app default
		key, err := StoreFile(r, "file", storage, StoreOptions{
			Prefix:       "docs",
			AllowedTypes: []string{"text/plain"},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(key))
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "doc.txt")
	require.NoError(t, err)
	_, err = part.Write([]byte("plain text content"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestFullUploadFlow(t *testing.T) {
	s := newTestStorage(t)
	app := &App{storage: s, urlPrefix: "/media/", dir: s.root}

	r := chi.NewRouter()
	r.Use(app.Middleware()...)
	app.Routes(r)

	// Upload handler
	r.Post("/upload", func(w http.ResponseWriter, r *http.Request) {
		storage := StorageFromContext(r.Context())
		key, err := StoreFile(r, "file", storage, StoreOptions{Prefix: "uploads"})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(key))
	})

	// Upload a file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "doc.txt")
	require.NoError(t, err)
	fileContent := []byte("document content")
	_, err = part.Write(fileContent)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	uploadReq := httptest.NewRequest(http.MethodPost, "/upload", body)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	r.ServeHTTP(uploadRec, uploadReq)

	require.Equal(t, http.StatusCreated, uploadRec.Code)
	key := uploadRec.Body.String()
	assert.Contains(t, key, "uploads/")

	// Fetch the uploaded file
	getReq := httptest.NewRequest(http.MethodGet, "/media/"+key, nil)
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)

	assert.Equal(t, http.StatusOK, getRec.Code)

	respBody, err := io.ReadAll(getRec.Body)
	require.NoError(t, err)
	assert.Equal(t, fileContent, respBody)
}
