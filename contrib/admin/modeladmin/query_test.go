package modeladmin

import (
	"context"
	"testing"

	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oliverandrich/burrow"
)

type testItem struct { //nolint:govet // fieldalignment: test struct
	document.Base
	Name   string `json:"name" form:"name"`
	Status string `json:"status" form:"status"`
}

func setupTestDB(t *testing.T) *den.DB {
	t.Helper()
	db := burrow.TestDB(t)

	err := den.Register(context.Background(), db, &testItem{})
	require.NoError(t, err)

	return db
}

func seedItems(t *testing.T, db *den.DB, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 1; i <= n; i++ {
		item := &testItem{Name: "Item " + string(rune('A'-1+i)), Status: "active"}
		err := den.Insert(ctx, db, item)
		require.NoError(t, err)
	}
}

func TestCreateItem(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	item := &testItem{Name: "Test", Status: "active"}
	err := createItem(ctx, db, item)
	require.NoError(t, err)
	assert.NotEmpty(t, item.ID)

	// Verify it was inserted.
	loaded, err := den.FindByID[testItem](ctx, db, item.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test", loaded.Name)
}

func TestGetItem(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	item := &testItem{Name: "Fetch Me", Status: "active"}
	err := den.Insert(ctx, db, item)
	require.NoError(t, err)

	loaded, err := getItem[testItem](ctx, db, item.ID)
	require.NoError(t, err)
	assert.Equal(t, "Fetch Me", loaded.Name)
}

func TestGetItem_NotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := getItem[testItem](ctx, db, "nonexistent")
	require.Error(t, err)
}

func TestUpdateItem(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	item := &testItem{Name: "Original", Status: "active"}
	err := den.Insert(ctx, db, item)
	require.NoError(t, err)

	item.Name = "Updated"
	err = updateItem(ctx, db, item)
	require.NoError(t, err)

	loaded, err := den.FindByID[testItem](ctx, db, item.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", loaded.Name)
}

func TestDeleteItem(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	item := &testItem{Name: "Delete Me", Status: "active"}
	err := den.Insert(ctx, db, item)
	require.NoError(t, err)

	err = deleteItem[testItem](ctx, db, item.ID)
	require.NoError(t, err)

	// Verify it was deleted.
	count, err := den.NewQuery[testItem](ctx, db).Count()
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
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
