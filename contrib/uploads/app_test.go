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
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestNewDefaults(t *testing.T) {
	app := New()
	assert.Equal(t, "data/uploads", app.dir)
	assert.Equal(t, "/uploads/", app.urlPrefix)
	assert.Nil(t, app.allowedTypes)
}

func TestNewWithBaseDir(t *testing.T) {
	app := New(WithBaseDir("/var/app"))
	assert.Equal(t, filepath.Join("/var/app", "uploads"), app.dir)
	assert.Equal(t, "/uploads/", app.urlPrefix)
}

func TestNewWithURLPrefix(t *testing.T) {
	app := New(WithURLPrefix("/media/"))
	assert.Equal(t, "data/uploads", app.dir)
	assert.Equal(t, "/media/", app.urlPrefix)
}

func TestNewWithAllowedTypes(t *testing.T) {
	app := New(WithAllowedTypes("image/jpeg", "image/png"))
	assert.Equal(t, []string{"image/jpeg", "image/png"}, app.AllowedTypes())
}

func TestNewCombinedOptions(t *testing.T) {
	app := New(
		WithBaseDir("./mydata"),
		WithURLPrefix("/files/"),
		WithAllowedTypes("image/png"),
	)
	assert.Equal(t, filepath.Join("./mydata", "uploads"), app.dir)
	assert.Equal(t, "/files/", app.urlPrefix)
	assert.Equal(t, []string{"image/png"}, app.allowedTypes)
}

// --- Configure / lifecycle tests ---

func configuredApp(t *testing.T, args ...string) *App {
	t.Helper()
	app := New()
	_ = app.Register(&burrow.AppConfig{})

	cmd := &cli.Command{
		Name:  "test",
		Flags: app.Flags(nil),
		Action: func(_ context.Context, cmd *cli.Command) error {
			return app.Configure(cmd)
		},
	}
	allArgs := append([]string{"test"}, args...)
	err := cmd.Run(t.Context(), allArgs)
	require.NoError(t, err)
	return app
}

func TestName(t *testing.T) {
	assert.Equal(t, "uploads", New().Name())
}

func TestConfigureCreatesStorage(t *testing.T) {
	app := configuredApp(t)
	require.NotNil(t, app.Store())
}

func TestConfigureDefaults(t *testing.T) {
	app := configuredApp(t)
	assert.Equal(t, "data/uploads", app.dir)
	assert.Equal(t, "/uploads/", app.urlPrefix)
	assert.Nil(t, app.allowedTypes)
}

func TestConfigureFlagOverrides(t *testing.T) {
	dir := t.TempDir()
	app := configuredApp(t,
		"--upload-dir", dir,
		"--upload-url-prefix", "/files/",
		"--upload-allowed-types", "image/jpeg, image/png",
	)
	assert.Equal(t, dir, app.dir)
	assert.Equal(t, "/files/", app.urlPrefix)
	assert.Equal(t, []string{"image/jpeg", "image/png"}, app.AllowedTypes())
	require.NotNil(t, app.Store())
}

func TestConfigureInvalidDir(t *testing.T) {
	// Point upload-dir at an invalid path to trigger NewLocalStorage error
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "afile")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0o644))

	app := New()
	_ = app.Register(&burrow.AppConfig{})

	cmd := &cli.Command{
		Name:  "test",
		Flags: app.Flags(nil),
		Action: func(_ context.Context, cmd *cli.Command) error {
			return app.Configure(cmd)
		},
	}
	err := cmd.Run(t.Context(), []string{"test", "--upload-dir", filepath.Join(filePath, "subdir")})
	require.Error(t, err)
}

func TestConfigureOptionDefaultsShownInFlags(t *testing.T) {
	app := New(WithBaseDir("/var/app"), WithURLPrefix("/media/"))
	flags := app.Flags(nil)

	var dirDefault, prefixDefault string
	for _, f := range flags {
		if sf, ok := f.(*cli.StringFlag); ok {
			switch sf.Name {
			case "upload-dir":
				dirDefault = sf.Value
			case "upload-url-prefix":
				prefixDefault = sf.Value
			}
		}
	}
	assert.Equal(t, filepath.Join("/var/app", "uploads"), dirDefault)
	assert.Equal(t, "/media/", prefixDefault)
}

func TestContextHelpers(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	t.Run("nil storage returns key", func(t *testing.T) {
		assert.Equal(t, "some/key.jpg", URL(ctx, "some/key.jpg"))
	})

	t.Run("storage from context", func(t *testing.T) {
		ctx := WithStorage(ctx, s) //nolint:govet // intentional shadow for test clarity
		got := Storage(ctx)
		require.NotNil(t, got)
		assert.Equal(t, "/media/some/key.jpg", URL(ctx, "some/key.jpg"))
	})
}

func TestMiddlewareInjectsStorage(t *testing.T) {
	s := newTestStorage(t)

	app := &App{storage: s, urlPrefix: "/media/", dir: s.root}
	mw := app.Middleware()
	require.Len(t, mw, 1)

	var gotStorage Store
	handler := mw[0](http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotStorage = Storage(r.Context())
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/media/"+key, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
	assert.Contains(t, rec.Body.String(), "served content")
}

func TestDefaultAllowedTypesApplied(t *testing.T) {
	s := newTestStorage(t)
	app := &App{storage: s, urlPrefix: "/media/", dir: s.root, allowedTypes: []string{"image/png"}}

	r := chi.NewRouter()
	r.Use(app.Middleware()...)

	r.Post("/upload", func(w http.ResponseWriter, r *http.Request) {
		storage := Storage(r.Context())
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/upload", body)
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
		storage := Storage(r.Context())
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/upload", body)
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
		storage := Storage(r.Context())
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

	uploadReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/upload", body)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	r.ServeHTTP(uploadRec, uploadReq)

	require.Equal(t, http.StatusCreated, uploadRec.Code)
	key := uploadRec.Body.String()
	assert.Contains(t, key, "uploads/")

	// Fetch the uploaded file
	getReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/media/"+key, nil)
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)

	assert.Equal(t, http.StatusOK, getRec.Code)

	respBody, err := io.ReadAll(getRec.Body)
	require.NoError(t, err)
	assert.Equal(t, fileContent, respBody)
}
