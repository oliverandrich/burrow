package notes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/den"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// apiRouter mounts the notes JSON API behind a middleware that authenticates
// every request as testUser() (user-42), standing in for RequireAuth.
func apiRouter(t *testing.T, db *den.DB) chi.Router {
	t.Helper()
	app := &App{repo: NewRepository(db), db: db}
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithUser(req.Context(), testUser())))
		})
	})
	app.apiRoutes(r)
	return r
}

func apiDo(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequestWithContext(t.Context(), method, target, rdr)
	req.Header.Set("Accept", "application/json")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAPICRUDRoundTrip(t *testing.T) {
	r := apiRouter(t, openTestDB(t))

	// Create — UserID is taken from the auth context, not the request body.
	rec := apiDo(t, r, http.MethodPost, "/api/notes", `{"title":"First","content":"hello","user_id":"hacker"}`)
	require.Equal(t, http.StatusCreated, rec.Code)
	var created Note
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, "user-42", created.UserID, "owner comes from the authenticated user")
	assert.Equal(t, "First", created.Title)

	// List.
	rec = apiDo(t, r, http.MethodGet, "/api/notes", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var list burrow.PageResponse[Note]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list.Items, 1)

	// Get.
	rec = apiDo(t, r, http.MethodGet, "/api/notes/"+created.ID, "")
	assert.Equal(t, http.StatusOK, rec.Code)

	// PATCH only the title; content is omitted and must survive the partial merge.
	rec = apiDo(t, r, http.MethodPatch, "/api/notes/"+created.ID, `{"title":"Renamed"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	rec = apiDo(t, r, http.MethodGet, "/api/notes/"+created.ID, "")
	var reloaded Note
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &reloaded))
	assert.Equal(t, "Renamed", reloaded.Title)
	assert.Equal(t, "hello", reloaded.Content, "PATCH left the omitted content untouched")

	// Delete.
	rec = apiDo(t, r, http.MethodDelete, "/api/notes/"+created.ID, "")
	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Gone.
	rec = apiDo(t, r, http.MethodGet, "/api/notes/"+created.ID, "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPIRejectsMissingTitle(t *testing.T) {
	r := apiRouter(t, openTestDB(t))

	rec := apiDo(t, r, http.MethodPost, "/api/notes", `{"content":"no title"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "validation_failed")
}

func TestAPIScopedToOwner(t *testing.T) {
	db := openTestDB(t)
	// A note owned by someone else must stay invisible to user-42.
	require.NoError(t, den.Save(context.Background(), db, &Note{UserID: "other", Title: "secret"}))

	rec := apiDo(t, apiRouter(t, db), http.MethodGet, "/api/notes", "")
	var list burrow.PageResponse[Note]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	assert.Empty(t, list.Items, "the scope hides other users' notes")
}
