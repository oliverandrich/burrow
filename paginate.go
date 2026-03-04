package burrow

import (
	"net/http"
	"strconv"

	"github.com/uptrace/bun"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// PageRequest holds pagination parameters parsed from a query string.
type PageRequest struct { //nolint:govet // fieldalignment: readability over optimization
	Limit  int    // items per page (default 20, max 100)
	Cursor string // opaque cursor for cursor-based pagination (empty = first page)
	Page   int    // 1-based page number for offset-based pagination (0 = not used)
}

// Offset returns the SQL OFFSET for the current page.
// Page 0 and 1 both return offset 0.
func (pr PageRequest) Offset() int {
	if pr.Page <= 1 {
		return 0
	}
	return (pr.Page - 1) * pr.Limit
}

// PageResult holds pagination metadata returned alongside items.
type PageResult struct {
	NextCursor string `json:"next_cursor,omitempty"` // cursor for next page (empty = no more)
	PrevCursor string `json:"prev_cursor,omitempty"` // cursor for previous page (empty = first page)
	HasMore    bool   `json:"has_more"`              // convenience: NextCursor != ""
	// Offset-based fields (only populated when using offset pagination):
	Page       int `json:"page,omitempty"`        // current page number (1-based)
	TotalPages int `json:"total_pages,omitempty"` // total number of pages
	TotalCount int `json:"total_count,omitempty"` // total number of items
}

// PageResponse wraps items with pagination metadata for JSON APIs.
type PageResponse[T any] struct {
	Items      []T        `json:"items"`
	Pagination PageResult `json:"pagination"`
}

// ParsePageRequest extracts pagination parameters from the request query string.
// If both cursor and page are present, cursor takes precedence.
func ParsePageRequest(r *http.Request) PageRequest {
	q := r.URL.Query()

	limit := min(max(parseIntOr(q.Get("limit"), defaultLimit), 1), maxLimit)

	cursor := q.Get("cursor")
	var page int
	if cursor == "" {
		page = max(parseIntOr(q.Get("page"), 0), 0)
	}

	return PageRequest{
		Limit:  limit,
		Cursor: cursor,
		Page:   page,
	}
}

// ApplyCursor applies cursor-based pagination to a Bun SelectQuery.
// It orders by orderColumn DESC and fetches limit+1 rows to detect whether
// more items exist. Use TrimCursorResults to trim the extra item.
func ApplyCursor(q *bun.SelectQuery, pr PageRequest, orderColumn string) *bun.SelectQuery {
	if pr.Cursor != "" {
		q = q.Where("? < ?", bun.Ident(orderColumn), pr.Cursor)
	}
	return q.OrderExpr("? DESC", bun.Ident(orderColumn)).Limit(pr.Limit + 1)
}

// ApplyOffset applies offset-based pagination to a Bun SelectQuery.
func ApplyOffset(q *bun.SelectQuery, pr PageRequest) *bun.SelectQuery {
	return q.Limit(pr.Limit).Offset(pr.Offset())
}

// TrimCursorResults trims a result slice that was fetched with limit+1.
// It returns the trimmed slice and whether more items exist beyond the limit.
func TrimCursorResults[T any](items []T, limit int) ([]T, bool) {
	if len(items) > limit {
		return items[:limit], true
	}
	return items, false
}

// CursorResult builds a PageResult for cursor-based pagination.
// lastCursor is the cursor value of the last item in the current page.
// hasMore indicates whether there are more items after this page.
func CursorResult(lastCursor string, hasMore bool) PageResult {
	if !hasMore {
		return PageResult{HasMore: false}
	}
	return PageResult{
		NextCursor: lastCursor,
		HasMore:    true,
	}
}

// OffsetResult builds a PageResult for offset-based pagination.
func OffsetResult(pr PageRequest, totalCount int) PageResult {
	totalPages := 0
	if pr.Limit > 0 && totalCount > 0 {
		totalPages = (totalCount + pr.Limit - 1) / pr.Limit
	}

	page := max(pr.Page, 1)

	return PageResult{
		Page:       page,
		TotalCount: totalCount,
		TotalPages: totalPages,
		HasMore:    page < totalPages,
	}
}

func parseIntOr(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}
