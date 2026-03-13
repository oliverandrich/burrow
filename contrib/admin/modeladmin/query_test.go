package modeladmin

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	"github.com/uptrace/bun/driver/sqliteshim"

	"github.com/oliverandrich/burrow"
)

type testItem struct { //nolint:govet // fieldalignment: test struct
	bun.BaseModel `bun:"table:items"`
	ID            int64  `bun:",pk,autoincrement"`
	Name          string `bun:",notnull" form:"name"`
	Status        string `bun:",notnull,default:'active'" form:"status"`
}

func setupTestDB(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?_pragma=foreign_keys(1)")
	require.NoError(t, err)
	db := bun.NewDB(sqldb, sqlitedialect.New())
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	_, err = db.NewCreateTable().Model((*testItem)(nil)).Exec(ctx)
	require.NoError(t, err)

	return db
}

func seedItems(t *testing.T, db *bun.DB, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 1; i <= n; i++ {
		item := &testItem{Name: "Item " + string(rune('A'-1+i)), Status: "active"}
		_, err := db.NewInsert().Model(item).Exec(ctx)
		require.NoError(t, err)
	}
}

func TestCreateItem(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	item := &testItem{Name: "Test", Status: "active"}
	err := createItem(ctx, db, item)
	require.NoError(t, err)
	assert.NotZero(t, item.ID)

	// Verify it was inserted.
	var loaded testItem
	err = db.NewSelect().Model(&loaded).Where("id = ?", item.ID).Scan(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Test", loaded.Name)
}

func TestGetItem(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	item := &testItem{Name: "Fetch Me", Status: "active"}
	_, err := db.NewInsert().Model(item).Exec(ctx)
	require.NoError(t, err)

	loaded, err := getItem[testItem](ctx, db, "1", nil)
	require.NoError(t, err)
	assert.Equal(t, "Fetch Me", loaded.Name)
}

func TestGetItem_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := getItem[testItem](ctx, db, "999", nil)
	require.Error(t, err)
}

func TestUpdateItem(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	item := &testItem{Name: "Original", Status: "active"}
	_, err := db.NewInsert().Model(item).Exec(ctx)
	require.NoError(t, err)

	item.Name = "Updated"
	err = updateItem(ctx, db, item)
	require.NoError(t, err)

	var loaded testItem
	err = db.NewSelect().Model(&loaded).Where("id = ?", item.ID).Scan(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Updated", loaded.Name)
}

func TestDeleteItem(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	item := &testItem{Name: "Delete Me", Status: "active"}
	_, err := db.NewInsert().Model(item).Exec(ctx)
	require.NoError(t, err)

	err = deleteItem[testItem](ctx, db, "1")
	require.NoError(t, err)

	// Verify it was deleted.
	count, err := db.NewSelect().Model((*testItem)(nil)).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestListItems_Pagination(t *testing.T) {
	db := setupTestDB(t)
	seedItems(t, db, 10)

	pr := burrow.PageRequest{Limit: 3, Page: 1}
	opts := listOpts{orderBy: "id ASC"}
	items, page, err := listItems[testItem](context.Background(), db, opts, pr)
	require.NoError(t, err)

	assert.Len(t, items, 3)
	assert.Equal(t, 10, page.TotalCount)
	assert.Equal(t, 4, page.TotalPages)
	assert.True(t, page.HasMore)
	assert.Equal(t, 1, page.Page)
}

func TestListItems_LastPage(t *testing.T) {
	db := setupTestDB(t)
	seedItems(t, db, 10)

	pr := burrow.PageRequest{Limit: 3, Page: 4}
	opts := listOpts{orderBy: "id ASC"}
	items, page, err := listItems[testItem](context.Background(), db, opts, pr)
	require.NoError(t, err)

	assert.Len(t, items, 1)
	assert.Equal(t, 4, page.Page)
	assert.False(t, page.HasMore)
}

func TestListItems_Empty(t *testing.T) {
	db := setupTestDB(t)

	pr := burrow.PageRequest{Limit: 10, Page: 1}
	items, page, err := listItems[testItem](context.Background(), db, listOpts{}, pr)
	require.NoError(t, err)

	assert.Empty(t, items)
	assert.Equal(t, 0, page.TotalCount)
}
