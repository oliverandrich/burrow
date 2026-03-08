package bootstrap

import (
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
)

func TestPageNumbers(t *testing.T) {
	tests := []struct { //nolint:govet // fieldalignment: readability over optimization
		name    string
		current int
		total   int
		want    []int
	}{
		{"3 pages", 1, 3, []int{1, 2, 3}},
		{"7 pages no ellipsis", 4, 7, []int{1, 2, 3, 4, 5, 6, 7}},
		{"10 pages first page", 1, 10, []int{1, 2, -1, 10}},
		{"10 pages middle", 5, 10, []int{1, -1, 4, 5, 6, -1, 10}},
		{"10 pages last page", 10, 10, []int{1, -1, 9, 10}},
		{"10 pages near start", 3, 10, []int{1, 2, 3, 4, -1, 10}},
		{"10 pages near end", 8, 10, []int{1, -1, 7, 8, 9, 10}},
		{"1 page", 1, 1, []int{1}},
		{"2 pages", 1, 2, []int{1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pageNumbers(tt.current, tt.total)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPageLimit(t *testing.T) {
	tests := []struct {
		name       string
		totalCount int
		totalPages int
		want       int
	}{
		{"standard", 25, 3, 9},
		{"exact", 20, 2, 10},
		{"zero pages", 0, 0, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pageLimit(burrow.PageResult{TotalCount: tt.totalCount, TotalPages: tt.totalPages})
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPageURL(t *testing.T) {
	assert.Equal(t, "/notes?page=2&limit=20", pageURL("/notes", 2, 20))
	assert.Equal(t, "/admin/notes?page=1&limit=10", pageURL("/admin/notes", 1, 10))
}
