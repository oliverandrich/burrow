package burrow

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testItem is a simple document type used in fresh DB tests.
type testItem struct {
	document.Base
	Name string `json:"name"`
}

func TestFreshDB_RegisterDocumentsOnEmptyDatabase(t *testing.T) {
	db := testDB(t)

	err := den.Register(t.Context(), db, &testItem{})
	require.NoError(t, err)

	// Verify the table accepts inserts.
	item := &testItem{Name: "test"}
	err = den.Insert(t.Context(), db, item)
	require.NoError(t, err)
	assert.NotEmpty(t, item.ID)
}

func TestFreshDB_EmptyTableReturnsEmptyResults(t *testing.T) {
	db := testDB(t)

	err := den.Register(t.Context(), db, &testItem{})
	require.NoError(t, err)

	// Query the empty table — should return zero items, not an error.
	items, err := den.NewQuery[testItem](db).All(t.Context())
	require.NoError(t, err)
	assert.Empty(t, items)
}

// docApp is a test helper implementing App + HasDocuments.
type docApp struct {
	name string
	docs []any
}

func (a *docApp) Name() string     { return a.name }
func (a *docApp) Documents() []any { return a.docs }

func TestFreshDB_ServerBootstrapWithMultipleApps(t *testing.T) {
	appA := &docApp{name: "app_a", docs: []any{&testItem{}}}
	appB := &minimalApp{} // no documents

	srv := NewServer(appA, appB)
	db := testDB(t)

	err := srv.bootstrap(t.Context(), db, nil)
	require.NoError(t, err)

	// Verify that app_a's document type was registered (table exists).
	item := &testItem{Name: "hello"}
	err = den.Insert(t.Context(), db, item)
	require.NoError(t, err)

	// Verify both apps were registered.
	apps := srv.Registry().Apps()
	require.Len(t, apps, 2)
}

func TestFreshDB_ServerBootstrapWithNoDocuments(t *testing.T) {
	app := &minimalApp{}
	srv := NewServer(app)
	db := testDB(t)

	err := srv.bootstrap(t.Context(), db, nil)
	require.NoError(t, err)

	// Verify the app was registered successfully.
	apps := srv.Registry().Apps()
	require.Len(t, apps, 1)
	assert.Equal(t, "minimal", apps[0].Name())
}

func TestFreshDB_EmptyListEndpointReturnsOK(t *testing.T) {
	db := testDB(t)
	err := den.Register(t.Context(), db, &testItem{})
	require.NoError(t, err)

	// Build a handler that counts items from the empty table.
	r := chi.NewRouter()
	r.Get("/items", Handle(func(w http.ResponseWriter, r *http.Request) error {
		count, err := den.NewQuery[testItem](db).Count(r.Context())
		if err != nil {
			return NewHTTPError(http.StatusInternalServerError, "query failed")
		}
		return JSON(w, http.StatusOK, map[string]int64{"count": count})
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"count":0`)
}

func TestFreshDB_RegisterDocumentsIdempotent(t *testing.T) {
	db := testDB(t)

	// Register twice — second call should be idempotent.
	err := den.Register(t.Context(), db, &testItem{})
	require.NoError(t, err)

	err = den.Register(t.Context(), db, &testItem{})
	require.NoError(t, err)

	// Verify the table still works.
	item := &testItem{Name: "test"}
	err = den.Insert(t.Context(), db, item)
	require.NoError(t, err)
}

func TestFreshDB_BootstrapAndHandleRequestsCleanly(t *testing.T) {
	appA := &docApp{name: "things", docs: []any{&testItem{}}}
	srv := NewServer(appA)
	db := testDB(t)

	err := srv.bootstrap(t.Context(), db, nil)
	require.NoError(t, err)

	// Simulate a request to a fresh (empty) table.
	r := chi.NewRouter()
	r.Get("/things", Handle(func(w http.ResponseWriter, r *http.Request) error {
		count, err := den.NewQuery[testItem](db).Count(r.Context())
		if err != nil {
			return NewHTTPError(http.StatusInternalServerError, "query failed")
		}
		return JSON(w, http.StatusOK, map[string]int64{"count": count})
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/things", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"count":0`)
}
