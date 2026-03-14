package burrow

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePageRequest_Defaults(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
	pr := ParsePageRequest(r)

	assert.Equal(t, 20, pr.Limit)
	assert.Equal(t, 0, pr.Page)
}

func TestParsePageRequest_CustomLimit(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items?limit=50", nil)
	pr := ParsePageRequest(r)

	assert.Equal(t, 50, pr.Limit)
}

func TestParsePageRequest_LimitClamping(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected int
	}{
		{"zero clamps to minimum", "limit=0", 1},
		{"negative clamps to minimum", "limit=-5", 1},
		{"over max clamps to max", "limit=200", 100},
		{"at max stays", "limit=100", 100},
		{"invalid clamps to default", "limit=abc", 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items?"+tt.query, nil)
			pr := ParsePageRequest(r)
			assert.Equal(t, tt.expected, pr.Limit)
		})
	}
}

func TestParsePageRequest_Page(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items?page=3&limit=25", nil)
	pr := ParsePageRequest(r)

	assert.Equal(t, 3, pr.Page)
	assert.Equal(t, 25, pr.Limit)
}

func TestParsePageRequest_InvalidPage(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected int
	}{
		{"zero stays zero", "page=0", 0},
		{"negative resets to zero", "page=-1", 0},
		{"non-numeric resets to zero", "page=abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items?"+tt.query, nil)
			pr := ParsePageRequest(r)
			assert.Equal(t, tt.expected, pr.Page)
		})
	}
}

func TestOffsetResult(t *testing.T) {
	tests := []struct {
		name       string
		page       int
		limit      int
		totalCount int
		wantPage   int
		wantTotal  int
		wantPages  int
		wantMore   bool
	}{
		{
			name: "first page of many",
			page: 1, limit: 10, totalCount: 25,
			wantPage: 1, wantTotal: 25, wantPages: 3, wantMore: true,
		},
		{
			name: "middle page",
			page: 2, limit: 10, totalCount: 25,
			wantPage: 2, wantTotal: 25, wantPages: 3, wantMore: true,
		},
		{
			name: "last page",
			page: 3, limit: 10, totalCount: 25,
			wantPage: 3, wantTotal: 25, wantPages: 3, wantMore: false,
		},
		{
			name: "exact page boundary",
			page: 2, limit: 10, totalCount: 20,
			wantPage: 2, wantTotal: 20, wantPages: 2, wantMore: false,
		},
		{
			name: "single page",
			page: 1, limit: 10, totalCount: 5,
			wantPage: 1, wantTotal: 5, wantPages: 1, wantMore: false,
		},
		{
			name: "empty result",
			page: 1, limit: 10, totalCount: 0,
			wantPage: 1, wantTotal: 0, wantPages: 0, wantMore: false,
		},
		{
			name: "page 0 treated as page 1",
			page: 0, limit: 10, totalCount: 25,
			wantPage: 1, wantTotal: 25, wantPages: 3, wantMore: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PageRequest{Limit: tt.limit, Page: tt.page}
			result := OffsetResult(pr, tt.totalCount)

			assert.Equal(t, tt.wantPage, result.Page)
			assert.Equal(t, tt.wantTotal, result.TotalCount)
			assert.Equal(t, tt.wantPages, result.TotalPages)
			assert.Equal(t, tt.wantMore, result.HasMore)
		})
	}
}

func TestPageRequest_Offset(t *testing.T) {
	tests := []struct {
		name   string
		page   int
		limit  int
		expect int
	}{
		{"page 1", 1, 10, 0},
		{"page 2", 2, 10, 10},
		{"page 3", 3, 25, 50},
		{"page 0 treated as 1", 0, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PageRequest{Page: tt.page, Limit: tt.limit}
			assert.Equal(t, tt.expect, pr.Offset())
		})
	}
}

func TestApplyOffset_Integration(t *testing.T) {
	db := TestDB(t)
	ctx := t.Context()

	_, err := db.ExecContext(ctx, `CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)

	for i := 1; i <= 10; i++ {
		_, err := db.ExecContext(ctx, `INSERT INTO items (id, name) VALUES (?, ?)`, i, "item")
		require.NoError(t, err)
	}

	type Item struct { //nolint:govet // test struct
		ID   int64 `bun:",pk"`
		Name string
	}

	t.Run("first page", func(t *testing.T) {
		pr := PageRequest{Limit: 3, Page: 1}
		var items []Item
		q := db.NewSelect().Model(&items).Order("id ASC")
		q = ApplyOffset(q, pr)
		err := q.Scan(ctx)
		require.NoError(t, err)

		assert.Len(t, items, 3)
		assert.Equal(t, int64(1), items[0].ID)
		assert.Equal(t, int64(3), items[2].ID)
	})

	t.Run("second page", func(t *testing.T) {
		pr := PageRequest{Limit: 3, Page: 2}
		var items []Item
		q := db.NewSelect().Model(&items).Order("id ASC")
		q = ApplyOffset(q, pr)
		err := q.Scan(ctx)
		require.NoError(t, err)

		assert.Len(t, items, 3)
		assert.Equal(t, int64(4), items[0].ID)
		assert.Equal(t, int64(6), items[2].ID)
	})

	t.Run("last partial page", func(t *testing.T) {
		pr := PageRequest{Limit: 3, Page: 4}
		var items []Item
		q := db.NewSelect().Model(&items).Order("id ASC")
		q = ApplyOffset(q, pr)
		err := q.Scan(ctx)
		require.NoError(t, err)

		assert.Len(t, items, 1)
		assert.Equal(t, int64(10), items[0].ID)
	})
}
