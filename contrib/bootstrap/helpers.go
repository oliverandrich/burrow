package bootstrap

import (
	"fmt"

	"github.com/oliverandrich/burrow"
)

// pageURL builds a pagination URL preserving the existing limit parameter.
func pageURL(baseURL string, page, limit int) string {
	return fmt.Sprintf("%s?page=%d&limit=%d", baseURL, page, limit)
}

// pageLimit derives the per-page limit from a PageResult.
func pageLimit(page burrow.PageResult) int {
	if page.TotalPages == 0 {
		return 20
	}
	return (page.TotalCount + page.TotalPages - 1) / page.TotalPages
}

// pageNumbers returns page numbers to display, using -1 for ellipsis gaps.
// Shows at most 7 slots: first, last, current, and neighbors with ellipsis.
func pageNumbers(current, total int) []int {
	if total <= 7 {
		pages := make([]int, total)
		for i := range total {
			pages[i] = i + 1
		}
		return pages
	}

	pages := make([]int, 0, 7)
	pages = append(pages, 1)

	if current > 3 {
		pages = append(pages, -1) // ellipsis
	}

	// Window around current page.
	for p := max(2, current-1); p <= min(total-1, current+1); p++ {
		pages = append(pages, p)
	}

	if current < total-2 {
		pages = append(pages, -1) // ellipsis
	}

	pages = append(pages, total)
	return pages
}
