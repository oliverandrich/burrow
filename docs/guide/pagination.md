# Pagination

Burrow provides offset-based pagination utilities for numbered pages. All types and functions are in the root `burrow` package.

## Types

### PageRequest

Parsed from query parameters via `ParsePageRequest(r)`:

```go
type PageRequest struct {
    Limit int // items per page (default 20, max 100)
    Page  int // 1-based page number for offset-based pagination
}
```

Query parameters: `?limit=20&page=2`.

### PageResult

Returned alongside query results:

```go
type PageResult struct {
    HasMore    bool // convenience: more pages exist
    Page       int  // current page number (1-based)
    TotalPages int  // total number of pages
    TotalCount int  // total number of items
}
```

## Offset-Based Pagination

Best for admin panels, tables, and infinite scroll where users need to jump to specific pages or load the next page.

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

## Building Pagination URLs

`PageURL` builds a URL that preserves existing query parameters (search terms, filters, sort order) and sets the `page` parameter. This prevents pagination links from dropping the current query state.

```go
func PageURL(basePath, rawQuery string, page int) string
```

```go
// In a handler:
rawQuery := r.URL.RawQuery // e.g. "q=alpha&sort=-created_at"
url := burrow.PageURL("/notes", rawQuery, 3)
// => "/notes?page=3&q=alpha&sort=-created_at"
```

In templates, pass the current query string and use `pageURL` to generate links:

```html
{{- $base := "/notes" }}
{{- $query := .Query }}
{{- range pageNumbers .Page.Page .Page.TotalPages }}
<a href="{{ pageURL $base $query . }}">{{ . }}</a>
{{- end }}
```

## Bootstrap Pagination Component

The `contrib/bootstrap/templates` package provides a ready-made Bootstrap 5 pagination nav. See [Bootstrap — Pagination](../contrib/bootstrap.md#pagination).

Use the `bootstrap/pagination` template in your own templates. Pass a struct or map with `BasePath`, `RawQuery`, and `Page`:

```html
{{ template "bootstrap/pagination" dict "BasePath" "/notes" "RawQuery" .RawQuery "Page" .Page }}
```

This renders a `<nav>` with numbered page links, previous/next buttons, and ellipsis for large page counts. The current page is highlighted with Bootstrap's `active` class. Query parameters (search terms, filters) are preserved in pagination links.

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
