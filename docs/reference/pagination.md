# pagination — page request parsing, offset metadata, URL building

The `burrow/pagination` package holds the offset-pagination primitives: the request struct parsed from a query string, the result struct that carries computed metadata back to templates, the helpers that build paginator URLs and the truncated page-number list a UI uses.

The burrow root re-exports every name as either an alias or a wrapper, so apps that paginate occasionally can stay on `burrow.ParsePageRequest` / `burrow.PageResult`. In a package that paginates several lists, the direct `pagination.ParsePageRequest` spelling reads cleaner once `burrow/pagination` is in the import list.

Full signatures live on [pkg.go.dev](https://pkg.go.dev/github.com/oliverandrich/burrow/pagination). This page is a symbol index.

## Request + result types

| Symbol | Notes |
|---|---|
| `PageRequest` | Parsed `?limit=&page=` query — exposes `Limit`, `Page`, `Offset()` |
| `PageResult` | Computed metadata — `Total`, `Page`, `Limit`, `HasPrev`, `HasNext`, `PrevPage`, `NextPage` |
| `PageResponse[T any]` | Generic envelope for JSON APIs — `Items []T` + `Pagination PageResult` |

## Helpers

| Symbol | Notes |
|---|---|
| `ParsePageRequest(r)` | Read `?limit=` and `?page=` from a request; clamps to defaults |
| `OffsetResult(pr, totalCount)` | Build `PageResult` after running the count query |
| `PageURL(basePath, rawQuery, page)` | Build the URL for a target page, preserving every other query parameter (search filters, sort keys) |
| `PageNumbers(current, total)` | Compact truncated page-list for paginator UIs — e.g. `[1, …, 4, 5, 6, …, 20]` |

See the [Pagination guide](../guide/pagination.md) for the integration pattern with Den's `QuerySet` (limit + skip + count) and template usage.
