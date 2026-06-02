package crud_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/crud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func itemIDs(resp burrow.PageResponse[widget]) []string {
	ids := make([]string, len(resp.Items))
	for i, w := range resp.Items {
		ids[i] = w.ID
	}
	return ids
}

func TestCursorOptInSwitchesMode(t *testing.T) {
	db := newDB(t)
	for i := range 3 {
		save(t, db, &widget{Name: fmt.Sprintf("w%d", i)})
	}

	// Offset (default): total_count present, no cursor.
	off := listResp(t, crud.NewResource[widget](db), "/widgets?limit=2", nil)
	assert.Equal(t, 3, off.Pagination.TotalCount)
	assert.Empty(t, off.Pagination.NextCursor)

	// Cursor mode: a forward cursor, and total_count is intentionally absent.
	cur := listResp(t, crud.NewResource[widget](db, crud.WithCursorPagination[widget]()), "/widgets?limit=2", nil)
	assert.Equal(t, 0, cur.Pagination.TotalCount, "cursor mode does not run COUNT")
	assert.True(t, cur.Pagination.HasMore)
	assert.NotEmpty(t, cur.Pagination.NextCursor)
}

func TestCursorPaginatesForward(t *testing.T) {
	db := newDB(t)
	const n = 5
	for i := range n {
		save(t, db, &widget{Name: fmt.Sprintf("w%d", i)})
	}
	rs := crud.NewResource[widget](db, crud.WithCursorPagination[widget]())

	var seen []string
	cursor := ""
	for range n + 2 { // generous upper bound; loop breaks on !HasMore
		target := "/widgets?limit=2"
		if cursor != "" {
			target += "&after=" + cursor
		}
		resp := listResp(t, rs, target, nil)
		assert.LessOrEqual(t, len(resp.Items), 2, "limit is respected")
		seen = append(seen, itemIDs(resp)...)
		if !resp.Pagination.HasMore {
			assert.Empty(t, resp.Pagination.NextCursor, "no cursor past the last page")
			break
		}
		require.NotEmpty(t, resp.Pagination.NextCursor)
		cursor = resp.Pagination.NextCursor
	}

	require.Len(t, seen, n, "every row seen exactly once across pages")
	assert.True(t, sort.StringsAreSorted(seen), "cursor walks ids in ascending order")
	uniq := map[string]bool{}
	for _, id := range seen {
		uniq[id] = true
	}
	assert.Len(t, uniq, n, "no row appears on two pages")
}

func TestCursorComposesWithFilter(t *testing.T) {
	db := newDB(t)
	for i := range 4 {
		save(t, db, &widget{Name: "keep", Price: i})
	}
	save(t, db, &widget{Name: "drop"})

	rs := crud.NewResource[widget](db,
		crud.WithFilter[widget]("name"),
		crud.WithCursorPagination[widget](),
	)

	var seen int
	cursor := ""
	for range 6 {
		target := "/widgets?limit=2&name=keep"
		if cursor != "" {
			target += "&after=" + cursor
		}
		resp := listResp(t, rs, target, nil)
		for _, w := range resp.Items {
			assert.Equal(t, "keep", w.Name, "filter still applies in cursor mode")
		}
		seen += len(resp.Items)
		if !resp.Pagination.HasMore {
			break
		}
		cursor = resp.Pagination.NextCursor
	}
	assert.Equal(t, 4, seen, "only the filtered rows are paged")
}

func TestCursorLimitClamped(t *testing.T) {
	db := newDB(t)
	for i := range 3 {
		save(t, db, &widget{Name: fmt.Sprintf("w%d", i)})
	}
	rs := crud.NewResource[widget](db, crud.WithCursorPagination[widget]())

	// limit=0 is clamped to the default (>=1), so the page still returns rows.
	resp := listResp(t, rs, "/widgets?limit=0", nil)
	assert.NotEmpty(t, resp.Items, "limit is clamped to a sane minimum")
}

func TestCursorAfterUnknownIDReturnsTail(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{Name: "only"})
	rs := crud.NewResource[widget](db, crud.WithCursorPagination[widget]())

	// A cursor lexically below every id returns all rows; above, none.
	low := listResp(t, rs, "/widgets?after=0", nil)
	assert.Len(t, low.Items, 1)
	high := listResp(t, rs, "/widgets?after=ZZZZZZZZZZZZZZZZZZZZZZZZZZ", nil)
	assert.Empty(t, high.Items)
	assert.False(t, high.Pagination.HasMore)
}
