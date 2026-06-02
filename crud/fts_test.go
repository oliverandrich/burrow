package crud_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/burrowtest"
	"github.com/oliverandrich/burrow/crud"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"
	"github.com/oliverandrich/den/where"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// article carries full-text-indexed fields (den:"fts"), the prerequisite for
// WithFullTextSearch.
type article struct {
	document.Base
	OwnerID string `json:"owner_id" den:"index"`
	Title   string `json:"title" den:"fts"`
	Body    string `json:"body" den:"fts"`
}

func newArticleDB(t *testing.T) *den.DB {
	t.Helper()
	db := burrowtest.DB(t)
	require.NoError(t, den.Register(t.Context(), db, &article{}))
	return db
}

func saveArticle(t *testing.T, db *den.DB, a *article) {
	t.Helper()
	require.NoError(t, den.Save(t.Context(), db, a))
}

func mountArticles(rs *crud.Resource[article]) chi.Router {
	r := chi.NewRouter()
	r.Mount("/articles", rs)
	return r
}

func ftsResp(t *testing.T, rs *crud.Resource[article], target string) burrow.PageResponse[article] {
	t.Helper()
	rec := do(t, mountArticles(rs), http.MethodGet, target, "", nil)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	var resp burrow.PageResponse[article]
	decode(t, rec, &resp)
	return resp
}

func TestFTSFindsByToken(t *testing.T) {
	db := newArticleDB(t)
	saveArticle(t, db, &article{Title: "Go concurrency", Body: "goroutines and channels"})
	saveArticle(t, db, &article{Title: "Python basics", Body: "lists and dicts"})

	rs := crud.NewResource[article](db, crud.WithFullTextSearch[article]())
	resp := ftsResp(t, rs, "/articles?search=goroutines")
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "Go concurrency", resp.Items[0].Title)
}

func TestFTSMultiTokenAnd(t *testing.T) {
	db := newArticleDB(t)
	saveArticle(t, db, &article{Title: "alpha beta", Body: ""})
	saveArticle(t, db, &article{Title: "alpha only", Body: ""})

	rs := crud.NewResource[article](db, crud.WithFullTextSearch[article]())
	resp := ftsResp(t, rs, "/articles?search="+url.QueryEscape("alpha beta"))
	require.Len(t, resp.Items, 1, "both tokens must match (implicit AND)")
	assert.Equal(t, "alpha beta", resp.Items[0].Title)
}

func TestFTSSpecialCharsDoNotError(t *testing.T) {
	db := newArticleDB(t)
	saveArticle(t, db, &article{Title: "harmless", Body: "text"})

	rs := crud.NewResource[article](db, crud.WithFullTextSearch[article]())
	// Each of these would be an FTS5 syntax error if passed raw to MATCH.
	for _, term := range []string{`"`, `AND`, `foo"bar`, `*`, `title:x`, `(`} {
		rec := do(t, mountArticles(rs), http.MethodGet, "/articles?search="+url.QueryEscape(term), "", nil)
		assert.Equalf(t, http.StatusOK, rec.Code, "term %q must be sanitized, got %d: %s", term, rec.Code, rec.Body.String())
	}
}

func TestFTSEmptyTermFallsBackToList(t *testing.T) {
	db := newArticleDB(t)
	saveArticle(t, db, &article{Title: "one"})
	saveArticle(t, db, &article{Title: "two"})

	rs := crud.NewResource[article](db, crud.WithFullTextSearch[article]())
	resp := ftsResp(t, rs, "/articles?search=")
	assert.Len(t, resp.Items, 2, "empty term lists normally")
	assert.Equal(t, 2, resp.Pagination.TotalCount, "the normal list path carries a count")
}

func TestFTSComposesWithFilterAndScope(t *testing.T) {
	db := newArticleDB(t)
	saveArticle(t, db, &article{OwnerID: "alice", Title: "shared keyword"})
	saveArticle(t, db, &article{OwnerID: "bob", Title: "shared keyword"})

	rs := crud.NewResource[article](db,
		crud.WithScope[article](func(r *http.Request) []where.Condition {
			return []where.Condition{where.Field("owner_id").Eq(r.Header.Get("X-Owner"))}
		}),
		crud.WithFilter[article]("owner_id"),
		crud.WithFullTextSearch[article](),
	)
	rec := do(t, mountArticles(rs), http.MethodGet, "/articles?search=keyword", "", map[string]string{"X-Owner": "alice"})
	require.Equal(t, http.StatusOK, rec.Code)
	var resp burrow.PageResponse[article]
	decode(t, rec, &resp)
	require.Len(t, resp.Items, 1, "FTS results are still narrowed by scope")
	assert.Equal(t, "alice", resp.Items[0].OwnerID)
}

func TestFTSPaginationHasMoreNoTotalCount(t *testing.T) {
	db := newArticleDB(t)
	for i := range 5 {
		saveArticle(t, db, &article{Title: fmt.Sprintf("match %d", i), Body: "common"})
	}
	rs := crud.NewResource[article](db, crud.WithFullTextSearch[article]())

	resp := ftsResp(t, rs, "/articles?search=common&limit=2")
	assert.Len(t, resp.Items, 2)
	assert.True(t, resp.Pagination.HasMore, "more results exist")
	assert.Equal(t, 0, resp.Pagination.TotalCount, "FTS skips the COUNT")
}
