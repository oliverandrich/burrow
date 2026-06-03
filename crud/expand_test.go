package crud_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/burrowtest"
	"github.com/oliverandrich/burrow/crud"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type expAuthor struct {
	document.Base
	Name string `json:"name"`
}

type expPost struct {
	document.Base
	Title  string              `json:"title"`
	Author den.Link[expAuthor] `json:"author"`
}

// newPostDB registers both types and returns a db with one author + one post
// linking to it.
func newPostDB(t *testing.T) (*den.DB, *expPost) {
	t.Helper()
	db := burrowtest.DB(t)
	require.NoError(t, den.Register(t.Context(), db, &expAuthor{}, &expPost{}))

	author := &expAuthor{Name: "Ursula"}
	require.NoError(t, den.Save(t.Context(), db, author))
	post := &expPost{Title: "Earthsea", Author: den.NewLink(author)}
	require.NoError(t, den.Save(t.Context(), db, post))
	return db, post
}

func mountPosts(rs *crud.Resource[expPost]) chi.Router {
	r := chi.NewRouter()
	r.Mount("/posts", rs)
	return r
}

// getJSON does a GET and decodes the body into a generic map.
func getJSON(t *testing.T, h http.Handler, target string) map[string]any {
	t.Helper()
	rec := do(t, h, http.MethodGet, target, "", nil)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	var m map[string]any
	decode(t, rec, &m)
	return m
}

func TestExpandLoadsNestedObject(t *testing.T) {
	db, post := newPostDB(t)
	rs := crud.NewResource[expPost](db, crud.WithExpandable[expPost]("author"))

	got := getJSON(t, mountPosts(rs), "/posts/"+post.ID+"?expand=author")
	author, ok := got["author"].(map[string]any)
	require.True(t, ok, "author should expand to an object, got %T: %v", got["author"], got["author"])
	assert.Equal(t, "Ursula", author["name"])
}

func TestExpandWithoutOptionStaysBareID(t *testing.T) {
	db, post := newPostDB(t)
	// No WithExpandable: ?expand is ignored, author stays the bare id.
	rs := crud.NewResource[expPost](db)

	got := getJSON(t, mountPosts(rs), "/posts/"+post.ID+"?expand=author")
	_, isObj := got["author"].(map[string]any)
	assert.False(t, isObj, "author must stay a bare id without WithExpandable")
	assert.IsType(t, "", got["author"])
}

func TestExpandNotAllowlistedIgnored(t *testing.T) {
	db, post := newPostDB(t)
	rs := crud.NewResource[expPost](db, crud.WithExpandable[expPost]("author"))

	// Requesting a field that isn't allowlisted leaves everything as ids.
	got := getJSON(t, mountPosts(rs), "/posts/"+post.ID+"?expand=secret")
	assert.IsType(t, "", got["author"], "non-allowlisted expand is a no-op")
}

func TestExpandAbsentStaysBareID(t *testing.T) {
	db, post := newPostDB(t)
	rs := crud.NewResource[expPost](db, crud.WithExpandable[expPost]("author"))

	got := getJSON(t, mountPosts(rs), "/posts/"+post.ID)
	assert.IsType(t, "", got["author"], "no ?expand means no hydration")
}

func TestExpandUnknownAllowlistFieldWarns(t *testing.T) {
	db, _ := newPostDB(t)

	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(old)

	rs := crud.NewResource[expPost](db, crud.WithExpandable[expPost]("author", "bogus"))
	// A request triggers registration, where the allowlist is validated.
	do(t, mountPosts(rs), http.MethodGet, "/posts", "", nil)

	out := buf.String()
	assert.Contains(t, out, "bogus", "a non-link allowlist entry warns")
	assert.NotContains(t, out, "field=author", "a real link field does not warn")
}

func TestExpandList(t *testing.T) {
	db, _ := newPostDB(t)
	rs := crud.NewResource[expPost](db, crud.WithExpandable[expPost]("author"))

	var resp burrow.PageResponse[map[string]any]
	rec := do(t, mountPosts(rs), http.MethodGet, "/posts?expand=author", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	decode(t, rec, &resp)
	require.Len(t, resp.Items, 1)
	author, ok := resp.Items[0]["author"].(map[string]any)
	require.True(t, ok, "list items expand their links too")
	assert.Equal(t, "Ursula", author["name"])
}
