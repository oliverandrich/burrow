package modeladmin

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	"github.com/uptrace/bun/driver/sqliteshim"
)

// setupCascadeDB creates a schema with CASCADE and non-CASCADE foreign keys.
//
//	parents   <- children (ON DELETE CASCADE)
//	parents   <- friends  (ON DELETE SET NULL, no cascade)
func setupCascadeDB(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?_pragma=foreign_keys(1)")
	require.NoError(t, err)
	db := bun.NewDB(sqldb, sqlitedialect.New())
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()

	_, err = db.ExecContext(ctx, `CREATE TABLE parents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL
	)`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE children (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		parent_id INTEGER NOT NULL REFERENCES parents(id) ON DELETE CASCADE,
		label TEXT NOT NULL
	)`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE friends (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		parent_id INTEGER REFERENCES parents(id) ON DELETE SET NULL,
		note TEXT
	)`)
	require.NoError(t, err)

	return db
}

func TestDetectCascades_FindsCascadeOnly(t *testing.T) {
	db := setupCascadeDB(t)

	cascades := detectCascades(db, "parents")

	require.Len(t, cascades, 1)
	assert.Equal(t, "children", cascades[0].Table)
	assert.Equal(t, "parent_id", cascades[0].Column)
}

func TestDetectCascades_NoReferences(t *testing.T) {
	db := setupCascadeDB(t)

	cascades := detectCascades(db, "children")

	assert.Empty(t, cascades)
}

func TestDetectCascades_EmptyTableName(t *testing.T) {
	db := setupCascadeDB(t)

	cascades := detectCascades(db, "")

	assert.Empty(t, cascades)
}

func TestCountCascadeImpacts(t *testing.T) {
	db := setupCascadeDB(t)
	ctx := context.Background()

	// Insert a parent.
	_, err := db.ExecContext(ctx, `INSERT INTO parents (id, name) VALUES (1, 'Alice')`)
	require.NoError(t, err)

	// Insert children referencing the parent.
	_, err = db.ExecContext(ctx, `INSERT INTO children (parent_id, label) VALUES (1, 'c1'), (1, 'c2'), (1, 'c3')`)
	require.NoError(t, err)

	cascades := []cascadeRef{{Table: "children", Column: "parent_id"}}

	impacts, err := countCascadeImpacts(ctx, db, cascades, "1")
	require.NoError(t, err)
	require.Len(t, impacts, 1)
	assert.Equal(t, "children", impacts[0].Table)
	assert.Equal(t, 3, impacts[0].Count)
}

func TestCountCascadeImpacts_ZeroRows(t *testing.T) {
	db := setupCascadeDB(t)
	ctx := context.Background()

	// Insert a parent with no children.
	_, err := db.ExecContext(ctx, `INSERT INTO parents (id, name) VALUES (1, 'Alice')`)
	require.NoError(t, err)

	cascades := []cascadeRef{{Table: "children", Column: "parent_id"}}

	impacts, err := countCascadeImpacts(ctx, db, cascades, "1")
	require.NoError(t, err)
	// Zero-count impacts are omitted.
	assert.Empty(t, impacts)
}

func TestCountCascadeImpacts_MultipleCascades(t *testing.T) {
	db := setupCascadeDB(t)
	ctx := context.Background()

	// Add a second cascade table.
	_, err := db.ExecContext(ctx, `CREATE TABLE grandchildren (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		parent_id INTEGER NOT NULL REFERENCES parents(id) ON DELETE CASCADE,
		name TEXT
	)`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO parents (id, name) VALUES (1, 'Alice')`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO children (parent_id, label) VALUES (1, 'c1')`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO grandchildren (parent_id, name) VALUES (1, 'g1'), (1, 'g2')`)
	require.NoError(t, err)

	cascades := []cascadeRef{
		{Table: "children", Column: "parent_id"},
		{Table: "grandchildren", Column: "parent_id"},
	}

	impacts, err := countCascadeImpacts(ctx, db, cascades, "1")
	require.NoError(t, err)
	require.Len(t, impacts, 2)

	// Build a map for order-independent assertions.
	impactMap := make(map[string]int, len(impacts))
	for _, imp := range impacts {
		impactMap[imp.Table] = imp.Count
	}
	assert.Equal(t, 1, impactMap["children"])
	assert.Equal(t, 2, impactMap["grandchildren"])
}

func TestLookupTableDisplayName(t *testing.T) {
	t.Run("registered name", func(t *testing.T) {
		RegisterTableDisplayName("test_widgets", "Widgets")
		t.Cleanup(func() {
			tableDisplayMu.Lock()
			delete(tableDisplayNames, "test_widgets")
			tableDisplayMu.Unlock()
		})
		assert.Equal(t, "Widgets", lookupTableDisplayName("test_widgets"))
	})

	t.Run("unregistered falls back to table name", func(t *testing.T) {
		assert.Equal(t, "unknown_table", lookupTableDisplayName("unknown_table"))
	})

	t.Run("empty table and name are ignored", func(t *testing.T) {
		RegisterTableDisplayName("", "Empty")
		RegisterTableDisplayName("something", "")
		assert.Equal(t, "something", lookupTableDisplayName("something"))
	})
}

func TestHandleConfirmDelete(t *testing.T) {
	db, renderer, ma := setupHandlerTest(t)

	item := &testItem{Name: "Cascade Test", Status: "active"}
	_, err := db.NewInsert().Model(item).Exec(context.Background())
	require.NoError(t, err)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/items/%d/delete", item.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, renderer.confirmDeleteCalled)
	assert.NotNil(t, renderer.lastItem)
}

func TestHandleConfirmDelete_WithCascades(t *testing.T) {
	db, renderer, ma := setupHandlerTest(t)

	ctx := context.Background()

	// Create a child table with CASCADE FK.
	_, err := db.ExecContext(ctx, `CREATE TABLE item_comments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
		body TEXT
	)`)
	require.NoError(t, err)

	// Re-create the router to trigger detectCascades in Routes().
	item := &testItem{Name: "Parent", Status: "active"}
	_, err = db.NewInsert().Model(item).Exec(ctx)
	require.NoError(t, err)

	// Insert child rows.
	_, err = db.ExecContext(ctx, `INSERT INTO item_comments (item_id, body) VALUES (?, 'c1'), (?, 'c2')`, item.ID, item.ID)
	require.NoError(t, err)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/items/%d/delete", item.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, renderer.confirmDeleteCalled)
	require.Len(t, renderer.lastConfig.DeleteImpacts, 1)
	assert.Equal(t, "item_comments", renderer.lastConfig.DeleteImpacts[0].Table)
	assert.Equal(t, "item_comments", renderer.lastConfig.DeleteImpacts[0].DisplayName) // no ModelAdmin registered → falls back to table name
	assert.Equal(t, 2, renderer.lastConfig.DeleteImpacts[0].Count)
}

func TestHandleConfirmDelete_NotFound(t *testing.T) {
	_, _, ma := setupHandlerTest(t)

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/999/delete", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleConfirmDelete_Forbidden(t *testing.T) {
	_, _, ma := setupHandlerTest(t)
	ma.CanDelete = false

	r := newRouter(ma)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items/1/delete", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Route is not registered when CanDelete is false, so 404.
	assert.Equal(t, http.StatusNotFound, w.Code)
}
