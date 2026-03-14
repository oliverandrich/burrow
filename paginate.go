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
type PageRequest struct {
	Limit int // items per page (default 20, max 100)
	Page  int // 1-based page number for offset-based pagination (0 = not used)
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
	HasMore    bool `json:"has_more"`              // convenience: more pages exist
	Page       int  `json:"page,omitempty"`        // current page number (1-based)
	TotalPages int  `json:"total_pages,omitempty"` // total number of pages
	TotalCount int  `json:"total_count,omitempty"` // total number of items
}

// PageResponse wraps items with pagination metadata for JSON APIs.
// Use with [OffsetResult] to populate the Pagination field:
//
//	return burrow.PageResponse[Item]{
//	    Items:      items,
//	    Pagination: burrow.OffsetResult(pr, totalCount),
//	}
type PageResponse[T any] struct {
	Items      []T        `json:"items"`
	Pagination PageResult `json:"pagination"`
}

// ParsePageRequest extracts pagination parameters from the request query string.
func ParsePageRequest(r *http.Request) PageRequest {
	q := r.URL.Query()

	limit := min(max(parseIntOr(q.Get("limit"), defaultLimit), 1), maxLimit)
	page := max(parseIntOr(q.Get("page"), 0), 0)

	return PageRequest{
		Limit: limit,
		Page:  page,
	}
}

// ApplyOffset applies offset-based pagination to a Bun SelectQuery.
func ApplyOffset(q *bun.SelectQuery, pr PageRequest) *bun.SelectQuery {
	return q.Limit(pr.Limit).Offset(pr.Offset())
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
