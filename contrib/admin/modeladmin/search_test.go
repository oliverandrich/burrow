package modeladmin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oliverandrich/den"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/forms"
)

func setupSearchDB(t *testing.T) *den.DB {
	t.Helper()
	db := burrow.TestDB(t)

	ctx := context.Background()
	err := den.Register(ctx, db, &testItem{})
	require.NoError(t, err)

	items := []testItem{
		{Name: "Alpha", Status: "active"},
		{Name: "Beta", Status: "active"},
		{Name: "Gamma", Status: "inactive"},
		{Name: "Delta", Status: "inactive"},
		{Name: "Alpha Beta", Status: "active"},
	}
	for i := range items {
		err := den.Insert(ctx, db, &items[i])
		require.NoError(t, err)
	}
	return db
}

func TestSearch_ByName(t *testing.T) {
	db := setupSearchDB(t)
	opts := listOpts{
		searchTerm:   "Alpha",
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
		searchTerm:   "Alpha",
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

func TestFilter_Select(t *testing.T) {
	db := setupSearchDB(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items?status=active", nil)
	opts := listOpts{
		filters: []FilterDef{
			{Field: "status", Type: "select", Choices: []forms.Choice{{Value: "active"}, {Value: "inactive"}}},
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items?status=invalid", nil)
	opts := listOpts{
		filters: []FilterDef{
			{Field: "status", Type: "select", Choices: []forms.Choice{{Value: "active"}, {Value: "inactive"}}},
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items?sort=name", nil)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items?sort=-name", nil)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items?sort=status", nil)
	opts := listOpts{
		sortFields: []string{"name"}, // "status" not in allowed list
		r:          req,
	}
	pr := burrow.PageRequest{Limit: 10, Page: 1}

	items, _, err := listItems[testItem](context.Background(), db, opts, pr)
	require.NoError(t, err)
	assert.Len(t, items, 5, "disallowed sort field should be ignored")
}
