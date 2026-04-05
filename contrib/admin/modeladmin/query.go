package modeladmin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/where"

	"github.com/oliverandrich/burrow"
)

// listOpts holds all options for listing items.
type listOpts struct { //nolint:govet // fieldalignment: readability over optimization
	orderBy      string
	searchTerm   string
	searchFields []string
	filters      []FilterDef
	sortFields   []string
	r            *http.Request
}

// listItems queries the database for a paginated list of items.
func listItems[T any](ctx context.Context, db *den.DB, opts listOpts, pr burrow.PageRequest) ([]T, burrow.PageResult, error) {
	conditions := buildConditions(opts)

	qs := den.NewQuery[T](ctx, db, conditions...)

	// Apply sorting: user-requested sort takes precedence over default.
	qs = applySorting(qs, opts)

	qs = qs.Limit(pr.Limit).Skip(pr.Offset())

	ptrs, totalCount, err := qs.AllWithCount()
	if err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("list items: %w", err)
	}

	items := derefSlice(ptrs)
	return items, burrow.OffsetResult(pr, int(totalCount)), nil
}

// allItems queries the database for all matching items without pagination.
// It applies search, filters, and sorting — same as listItems but
// returns the full result set for export use cases.
func allItems[T any](ctx context.Context, db *den.DB, opts listOpts) ([]T, error) {
	conditions := buildConditions(opts)

	qs := den.NewQuery[T](ctx, db, conditions...)
	qs = applySorting(qs, opts)

	ptrs, err := qs.All()
	if err != nil {
		return nil, fmt.Errorf("export items: %w", err)
	}

	return derefSlice(ptrs), nil
}

// getItem fetches a single item by primary key.
func getItem[T any](ctx context.Context, db *den.DB, id string) (*T, error) {
	item, err := den.FindByID[T](ctx, db, id)
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}
	return item, nil
}

// createItem inserts a new item into the database.
func createItem[T any](ctx context.Context, db *den.DB, item *T) error {
	if err := den.Insert(ctx, db, item); err != nil {
		return fmt.Errorf("create item: %w", err)
	}
	return nil
}

// updateItem updates an existing item in the database.
func updateItem[T any](ctx context.Context, db *den.DB, item *T) error {
	if err := den.Update(ctx, db, item); err != nil {
		return fmt.Errorf("update item: %w", err)
	}
	return nil
}

// deleteItem removes an item by primary key. Returns nil if the item does not exist.
func deleteItem[T any](ctx context.Context, db *den.DB, id string) error {
	item, err := den.FindByID[T](ctx, db, id)
	if err != nil {
		if errors.Is(err, den.ErrNotFound) {
			return nil // already deleted or never existed
		}
		return fmt.Errorf("delete item: %w", err)
	}
	if err := den.Delete(ctx, db, item); err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	return nil
}

// countItems returns the total number of items of type T.
func countItems[T any](ctx context.Context, db *den.DB) (int64, error) {
	return den.NewQuery[T](ctx, db).Count()
}

// buildConditions collects search and filter conditions from opts.
func buildConditions(opts listOpts) []where.Condition {
	var conditions []where.Condition

	// Search conditions.
	if searchConds := buildSearchConditions(opts.searchTerm, opts.searchFields); len(searchConds) > 0 {
		conditions = append(conditions, searchConds...)
	}

	// Filter conditions.
	if opts.r != nil {
		if filterConds := buildFilterConditions(opts.r, opts.filters); len(filterConds) > 0 {
			conditions = append(conditions, filterConds...)
		}
	}

	return conditions
}

// applySorting applies sort to the query set based on opts.
func applySorting[T any](qs den.QuerySet[T], opts listOpts) den.QuerySet[T] {
	sortApplied := false
	if opts.r != nil && len(opts.sortFields) > 0 {
		field, dir, ok := parseSortParam(opts.r, opts.sortFields)
		if ok {
			qs = qs.Sort(field, dir)
			sortApplied = true
		}
	}
	if !sortApplied && opts.orderBy != "" {
		// Parse "field DESC" or "field ASC" format.
		field, dir := parseOrderBy(opts.orderBy)
		if field != "" {
			qs = qs.Sort(field, dir)
		}
	}
	return qs
}

// derefSlice converts []*T to []T by dereferencing each pointer.
func derefSlice[T any](ptrs []*T) []T {
	if len(ptrs) == 0 {
		return nil
	}
	items := make([]T, len(ptrs))
	for i, p := range ptrs {
		items[i] = *p
	}
	return items
}

// parseOrderBy parses a SQL-style "field DESC" or "field ASC" string
// into a field name and sort direction.
func parseOrderBy(orderBy string) (string, den.SortDirection) {
	parts := strings.Fields(orderBy)
	if len(parts) == 0 {
		return "", den.Asc
	}
	field := parts[0]
	dir := den.Asc
	if len(parts) > 1 && strings.EqualFold(parts[1], "DESC") {
		dir = den.Desc
	}
	return field, dir
}
