package crud

import (
	"net/http"
	"net/url"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/where"
)

// WithCursorPagination switches the list endpoint to forward cursor pagination
// instead of the offset default. Clients page with ?after=<cursor>, following
// the next_cursor returned in each response; the cursor is a document id.
//
// Cursor mode orders strictly by document id ascending, so [WithSort] and the
// client's ?ordering do not apply (Den's cursor is id-keyset). Filtering and
// search still apply. It returns no total_count — that is the point: cursor
// pagination avoids the COUNT that degrades on large, append-only tables.
func WithCursorPagination[T any]() Option[T] {
	return func(rs *Resource[T]) { rs.cursor = true }
}

// listCursor serves one forward cursor page: rows after ?after (by id), ordered
// by id ascending, with next_cursor derived by over-fetching one extra row so
// has_more needs no COUNT. conds carries the already-built filter/search
// clauses; the default sort and ?ordering are intentionally ignored.
func (rs *Resource[T]) listCursor(w http.ResponseWriter, r *http.Request, params url.Values, conds []where.Condition) error {
	limit := burrow.ParsePageRequest(r).Limit

	q := den.NewQuery[T](rs.db, rs.scopeConds(r)...).
		Where(conds...).
		Sort(den.FieldID, den.Asc).
		Limit(limit + 1) // one extra row tells us whether another page exists
	if after := params.Get("after"); after != "" {
		q = q.After(after)
	}
	items, err := q.All(r.Context())
	if err != nil {
		return rs.fail(w, r, err)
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	next := ""
	if hasMore && len(items) > 0 {
		next = rs.stringFieldAt(items[len(items)-1], rs.baseID)
	}

	return burrow.JSON(w, http.StatusOK, burrow.PageResponse[any]{
		Items:      rs.views(items),
		Pagination: burrow.CursorResult(hasMore, next),
	})
}
