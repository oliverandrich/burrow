# Pagination

Burrow provides pagination utilities for both cursor-based (infinite scroll) and offset-based (numbered pages) patterns. All types and functions are in the root `burrow` package.

## Types

### PageRequest

Parsed from query parameters via `ParsePageRequest(r)`:

```go
type PageRequest struct {
    Limit  int    // items per page (default 20, max 100)
    Cursor string // opaque cursor for cursor-based pagination
    Page   int    // 1-based page number for offset-based pagination
}
```

Query parameters: `?limit=20&cursor=abc` or `?limit=20&page=2`. When both `cursor` and `page` are present, cursor takes precedence.

### PageResult

Returned alongside query results:

```go
type PageResult struct {
    NextCursor string // cursor for next page (empty = no more)
    PrevCursor string // cursor for previous page
    HasMore    bool   // convenience: more pages exist
    Page       int    // current page number (offset-based only)
    TotalPages int    // total number of pages (offset-based only)
    TotalCount int    // total number of items (offset-based only)
}
```

## Cursor-Based Pagination

Best for infinite scroll, feeds, and real-time data where items may be inserted or deleted between requests.

### Query Helper

`ApplyCursor` adds `WHERE`, `ORDER BY`, and `LIMIT` to a Bun query:

```go
func ApplyCursor(q *bun.SelectQuery, pr PageRequest, orderColumn string) *bun.SelectQuery
```

It fetches `limit + 1` rows to detect whether more items exist.

### Building Results

```go
// Trim the extra row and detect if there are more items.
items, hasMore := burrow.TrimCursorResults(notes, pr.Limit)

// Build the PageResult with the last item's cursor value.
var lastCursor string
if len(items) > 0 {
    lastCursor = strconv.FormatInt(items[len(items)-1].ID, 10)
}
pageResult := burrow.CursorResult(lastCursor, hasMore)
```

### Full Example

```go
func (r *Repository) ListPaged(ctx context.Context, userID int64, pr burrow.PageRequest) ([]Note, burrow.PageResult, error) {
    var notes []Note
    q := r.db.NewSelect().Model(&notes).Where("user_id = ?", userID)
    q = burrow.ApplyCursor(q, pr, "id")
    if err := q.Scan(ctx); err != nil {
        return nil, burrow.PageResult{}, err
    }

    notes, hasMore := burrow.TrimCursorResults(notes, pr.Limit)
    var lastID string
    if len(notes) > 0 {
        lastID = strconv.FormatInt(notes[len(notes)-1].ID, 10)
    }

    return notes, burrow.CursorResult(lastID, hasMore), nil
}
```

## Offset-Based Pagination

Best for admin panels and tables where users need to jump to specific pages.

### Query Helper

`ApplyOffset` adds `LIMIT` and `OFFSET` to a Bun query:

```go
func ApplyOffset(q *bun.SelectQuery, pr PageRequest) *bun.SelectQuery
```

### Building Results

`OffsetResult` computes page metadata from a total count:

```go
pageResult := burrow.OffsetResult(pr, totalCount)
```

### Full Example

```go
func (r *Repository) ListAllPaged(ctx context.Context, pr burrow.PageRequest) ([]Note, burrow.PageResult, error) {
    count, err := r.db.NewSelect().Model((*Note)(nil)).Count(ctx)
    if err != nil {
        return nil, burrow.PageResult{}, err
    }

    var notes []Note
    q := r.db.NewSelect().Model(&notes).Order("created_at DESC", "id DESC")
    q = burrow.ApplyOffset(q, pr)
    if err := q.Scan(ctx); err != nil {
        return nil, burrow.PageResult{}, err
    }

    return notes, burrow.OffsetResult(pr, count), nil
}
```

### Handler

```go
func (h *Handlers) AdminList(w http.ResponseWriter, r *http.Request) error {
    pr := burrow.ParsePageRequest(r)
    notes, page, err := h.repo.ListAllPaged(r.Context(), pr)
    if err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list notes")
    }
    // pass page to template...
}
```

## Bootstrap Pagination Component

The `contrib/bootstrap/templates` package provides a ready-made Bootstrap 5 pagination nav. See [Bootstrap — Pagination](../contrib/bootstrap.md#pagination).

Use the `bootstrap/pagination` template in your own templates:

```html
{{ template "bootstrap/pagination" .Page }}
```

This renders a `<nav>` with numbered page links, previous/next buttons, and ellipsis for large page counts. The current page is highlighted with Bootstrap's `active` class.

## JSON API

For JSON APIs, wrap results with `PageResponse`:

```go
type PageResponse[T any] struct {
    Items      []T        `json:"items"`
    Pagination PageResult `json:"pagination"`
}
```

```go
func (h *Handler) ListAPI(w http.ResponseWriter, r *http.Request) error {
    pr := burrow.ParsePageRequest(r)
    items, page, err := h.repo.ListPaged(r.Context(), pr)
    if err != nil {
        return err
    }
    return burrow.JSON(w, http.StatusOK, burrow.PageResponse[Item]{
        Items:      items,
        Pagination: page,
    })
}
```
