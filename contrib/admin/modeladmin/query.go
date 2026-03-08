package modeladmin

import (
	"context"
	"fmt"
	"net/http"

	"github.com/uptrace/bun"

	"github.com/oliverandrich/burrow"
)

// listOpts holds all options for listing items.
type listOpts struct { //nolint:govet // fieldalignment: readability over optimization
	relations    []string
	orderBy      string
	searchTerm   string
	searchFields []string
	filters      []FilterDef
	sortFields   []string
	r            *http.Request
}

// listItems queries the database for a paginated list of items.
func listItems[T any](ctx context.Context, db *bun.DB, opts listOpts, pr burrow.PageRequest) ([]T, burrow.PageResult, error) {
	var items []T
	q := db.NewSelect().Model(&items)

	for _, rel := range opts.relations {
		q = q.Relation(rel)
	}

	// Apply search.
	q = applySearch(q, opts.searchTerm, opts.searchFields)

	// Apply filters.
	if opts.r != nil {
		q = applyFilters(q, opts.r, opts.filters)
	}

	// Count after search/filter but before pagination.
	totalCount, err := q.Count(ctx)
	if err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("count items: %w", err)
	}

	// Apply sorting: user-requested sort takes precedence over default.
	sortApplied := false
	if opts.r != nil && len(opts.sortFields) > 0 {
		before := q
		q = applySort(q, opts.r, opts.sortFields)
		sortApplied = q != before
	}
	if !sortApplied && opts.orderBy != "" {
		q = q.OrderExpr(opts.orderBy)
	}

	q = burrow.ApplyOffset(q, pr)

	if err := q.Scan(ctx); err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("list items: %w", err)
	}

	return items, burrow.OffsetResult(pr, totalCount), nil
}

// getItem fetches a single item by primary key.
func getItem[T any](ctx context.Context, db *bun.DB, id string, relations []string) (*T, error) {
	item := new(T)
	q := db.NewSelect().Model(item).Where("id = ?", id)

	for _, rel := range relations {
		q = q.Relation(rel)
	}

	if err := q.Scan(ctx); err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}

	return item, nil
}

// createItem inserts a new item into the database.
func createItem[T any](ctx context.Context, db *bun.DB, item *T) error {
	if _, err := db.NewInsert().Model(item).Exec(ctx); err != nil {
		return fmt.Errorf("create item: %w", err)
	}
	return nil
}

// updateItem updates an existing item in the database.
func updateItem[T any](ctx context.Context, db *bun.DB, item *T) error {
	if _, err := db.NewUpdate().Model(item).WherePK().Exec(ctx); err != nil {
		return fmt.Errorf("update item: %w", err)
	}
	return nil
}

// deleteItem removes an item by primary key.
func deleteItem[T any](ctx context.Context, db *bun.DB, id string) error {
	item := new(T)
	if _, err := db.NewDelete().Model(item).Where("id = ?", id).Exec(ctx); err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	return nil
}
