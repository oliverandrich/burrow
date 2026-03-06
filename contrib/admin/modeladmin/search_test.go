package modeladmin

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	_ "modernc.org/sqlite"

	"codeberg.org/oliverandrich/burrow"
)

func setupSearchDB(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db := bun.NewDB(sqldb, sqlitedialect.New())
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	_, err = db.NewCreateTable().Model((*testItem)(nil)).Exec(ctx)
	require.NoError(t, err)

	items := []testItem{
		{Name: "Alpha", Status: "active"},
		{Name: "Beta", Status: "active"},
		{Name: "Gamma", Status: "inactive"},
		{Name: "Delta", Status: "inactive"},
		{Name: "Alpha Beta", Status: "active"},
	}
	for i := range items {
		_, err := db.NewInsert().Model(&items[i]).Exec(ctx)
		require.NoError(t, err)
	}
	return db
}

func TestSearch_ByName(t *testing.T) {
	db := setupSearchDB(t)
	opts := listOpts{
		searchTerm:   "alpha",
		searchFields: []string{"name"},
	}
	pr := burrow.PageRequest{Limit: 10, Page: 1}

	items, page, err := listItems[testItem](context.Background(), db, opts, pr)
	require.NoError(t, err)
	assert.Len(t, items, 2, "should find 'Alpha' and 'Alpha Beta'")
	assert.Equal(t, 2, page.TotalCount)
}

func TestSearch_EmptyTerm(t *testing.T) {
	db := setupSearchDB(t)
	opts := listOpts{
		searchTerm:   "",
		searchFields: []string{"name"},
	}
	pr := burrow.PageRequest{Limit: 10, Page: 1}

	items, _, err := listItems[testItem](context.Background(), db, opts, pr)
	require.NoError(t, err)
	assert.Len(t, items, 5, "empty search should return all items")
}

func TestSearch_NoFields(t *testing.T) {
	db := setupSearchDB(t)
	opts := listOpts{
		searchTerm:   "alpha",
		searchFields: nil,
	}
	pr := burrow.PageRequest{Limit: 10, Page: 1}

	items, _, err := listItems[testItem](context.Background(), db, opts, pr)
	require.NoError(t, err)
	assert.Len(t, items, 5, "no search fields should return all items")
}

func TestSearch_MultipleFields(t *testing.T) {
	db := setupSearchDB(t)
	opts := listOpts{
		searchTerm:   "active",
		searchFields: []string{"name", "status"},
	}
	pr := burrow.PageRequest{Limit: 10, Page: 1}

	items, _, err := listItems[testItem](context.Background(), db, opts, pr)
	require.NoError(t, err)
	// "active" matches status of Alpha, Beta, Alpha Beta (active)
	// and also "inactive" in Gamma, Delta
	assert.Len(t, items, 5)
}

func TestSearch_SQLInjectionSafety(t *testing.T) {
	db := setupSearchDB(t)
	opts := listOpts{
		searchTerm:   "'; DROP TABLE items; --",
		searchFields: []string{"name"},
	}
	pr := burrow.PageRequest{Limit: 10, Page: 1}

	items, _, err := listItems[testItem](context.Background(), db, opts, pr)
	require.NoError(t, err)
	assert.Empty(t, items, "SQL injection attempt should return no results")

	// Verify table still exists.
	count, err := db.NewSelect().Model((*testItem)(nil)).Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 5, count, "table should not be dropped")
}

func TestSearch_LikeWildcardEscaping(t *testing.T) {
	db := setupSearchDB(t)
	opts := listOpts{
		searchTerm:   "%",
		searchFields: []string{"name"},
	}
	pr := burrow.PageRequest{Limit: 10, Page: 1}

	items, _, err := listItems[testItem](context.Background(), db, opts, pr)
	require.NoError(t, err)
	assert.Empty(t, items, "literal % should not match anything")
}

func TestEscapeLike(t *testing.T) {
	assert.Equal(t, `hello`, escapeLike("hello"))
	assert.Equal(t, `he\%llo`, escapeLike("he%llo"))
	assert.Equal(t, `he\_llo`, escapeLike("he_llo"))
	assert.Equal(t, `he\\llo`, escapeLike(`he\llo`))
}

func TestFilter_Select(t *testing.T) {
	db := setupSearchDB(t)

	req := httptest.NewRequest(http.MethodGet, "/items?status=active", nil)
	opts := listOpts{
		filters: []FilterDef{
			{Field: "status", Type: "select", Choices: []Choice{{Value: "active"}, {Value: "inactive"}}},
		},
		r: req,
	}
	pr := burrow.PageRequest{Limit: 10, Page: 1}

	items, _, err := listItems[testItem](context.Background(), db, opts, pr)
	require.NoError(t, err)
	assert.Len(t, items, 3, "should find only active items")
}

func TestFilter_SelectInvalidChoice(t *testing.T) {
	db := setupSearchDB(t)

	req := httptest.NewRequest(http.MethodGet, "/items?status=invalid", nil)
	opts := listOpts{
		filters: []FilterDef{
			{Field: "status", Type: "select", Choices: []Choice{{Value: "active"}, {Value: "inactive"}}},
		},
		r: req,
	}
	pr := burrow.PageRequest{Limit: 10, Page: 1}

	items, _, err := listItems[testItem](context.Background(), db, opts, pr)
	require.NoError(t, err)
	assert.Len(t, items, 5, "invalid filter choice should be ignored")
}

func TestSort_Ascending(t *testing.T) {
	db := setupSearchDB(t)

	req := httptest.NewRequest(http.MethodGet, "/items?sort=name", nil)
	opts := listOpts{
		sortFields: []string{"name"},
		r:          req,
	}
	pr := burrow.PageRequest{Limit: 10, Page: 1}

	items, _, err := listItems[testItem](context.Background(), db, opts, pr)
	require.NoError(t, err)
	require.Len(t, items, 5)
	assert.Equal(t, "Alpha", items[0].Name)
	assert.Equal(t, "Alpha Beta", items[1].Name)
}

func TestSort_Descending(t *testing.T) {
	db := setupSearchDB(t)

	req := httptest.NewRequest(http.MethodGet, "/items?sort=-name", nil)
	opts := listOpts{
		sortFields: []string{"name"},
		r:          req,
	}
	pr := burrow.PageRequest{Limit: 10, Page: 1}

	items, _, err := listItems[testItem](context.Background(), db, opts, pr)
	require.NoError(t, err)
	require.Len(t, items, 5)
	assert.Equal(t, "Gamma", items[0].Name)
}

func TestSort_DisallowedField(t *testing.T) {
	db := setupSearchDB(t)

	req := httptest.NewRequest(http.MethodGet, "/items?sort=status", nil)
	opts := listOpts{
		sortFields: []string{"name"}, // "status" not in allowed list
		r:          req,
	}
	pr := burrow.PageRequest{Limit: 10, Page: 1}

	items, _, err := listItems[testItem](context.Background(), db, opts, pr)
	require.NoError(t, err)
	assert.Len(t, items, 5, "disallowed sort field should be ignored")
}
