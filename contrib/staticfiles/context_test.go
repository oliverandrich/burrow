package staticfiles

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestURL(t *testing.T) {
	app := mustNew(t, testFS)
	ctx := context.WithValue(context.Background(), ctxKeyApp{}, app)

	hash := contentHash([]byte("body{}"))
	assert.Equal(t, "/static/dist/styles."+hash+".css", URL(ctx, "dist/styles.css"))
}

func TestURLFallback(t *testing.T) {
	app := mustNew(t, testFS)
	ctx := context.WithValue(context.Background(), ctxKeyApp{}, app)

	// Unknown file returns prefix + name as-is.
	assert.Equal(t, "/static/dist/unknown.css", URL(ctx, "dist/unknown.css"))
}

func TestURLWithoutContext(t *testing.T) {
	// No app in context — returns name as-is (safe fallback, no prefix).
	ctx := context.Background()
	assert.Equal(t, "dist/styles.css", URL(ctx, "dist/styles.css"))
}

func TestURLCustomPrefix(t *testing.T) {
	app := mustNew(t, testFS, WithPrefix("/assets/"))
	ctx := context.WithValue(context.Background(), ctxKeyApp{}, app)

	hash := contentHash([]byte("body{}"))
	assert.Equal(t, "/assets/dist/styles."+hash+".css", URL(ctx, "dist/styles.css"))
}

func TestMiddlewareInjectsContext(t *testing.T) {
	app := mustNew(t, testFS)

	r := chi.NewRouter()
	for _, mw := range app.Middleware() {
		r.Use(mw)
	}

	var gotURL string
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		gotURL = URL(r.Context(), "dist/styles.css")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	hash := contentHash([]byte("body{}"))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/static/dist/styles."+hash+".css", gotURL)
}
