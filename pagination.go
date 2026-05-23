package burrow

import (
	"net/http"

	"github.com/oliverandrich/burrow/pagination"
)

// ParsePageRequest extracts limit and page from the request query string.
// Wrapper around [pagination.ParsePageRequest].
func ParsePageRequest(r *http.Request) PageRequest { return pagination.ParsePageRequest(r) }

// OffsetResult computes pagination metadata from a request and total count.
// Wrapper around [pagination.OffsetResult].
func OffsetResult(pr PageRequest, totalCount int) PageResult {
	return pagination.OffsetResult(pr, totalCount)
}

// PageURL builds a pagination URL preserving existing query parameters.
// Wrapper around [pagination.PageURL].
func PageURL(basePath, rawQuery string, page int) string {
	return pagination.PageURL(basePath, rawQuery, page)
}

// PageNumbers returns the truncated page list for a paginator UI. Wrapper
// around [pagination.PageNumbers].
func PageNumbers(current, total int) []int { return pagination.PageNumbers(current, total) }
