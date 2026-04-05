# Full-Text Search

!!! info "Prerequisites"
    This guide assumes familiarity with [Database](database.md) access and [Pagination](pagination.md). Read those first if you haven't already.

Den has built-in full-text search support. Mark fields with the `fts` tag option, and Den handles the FTS index creation and querying internally — for both SQLite (FTS5) and PostgreSQL (tsvector).

Compared to `LIKE '%term%'` queries, full-text search offers:

| Feature | `LIKE` | Full-Text Search |
|---------|--------|-----------------|
| Word-based matching | No (substring only) | Yes |
| Relevance ranking | No | Yes |
| Match highlighting | No | Yes (backend-dependent) |
| Performance at scale | O(n) full scan | O(log n) inverted index |

## Defining FTS Fields

Add the `fts` option to your `den` struct tags on fields that should be searchable:

```go
type Note struct {
    document.Base
    UserID  string `json:"user_id"`
    Title   string `json:"title" den:"index,fts"`
    Content string `json:"content" den:"fts"`
}
```

Den automatically creates and maintains the full-text search index when the document type is registered. No manual migration files, virtual tables, or triggers needed.

## Searching

Use `den.NewQuery` with `.Search()` to perform full-text queries:

```go
// Search across all FTS-indexed fields
results, err := den.NewQuery[Note](ctx, db).Search("hello world")

// Search with additional conditions
results, err := den.NewQuery[Note](ctx, db,
    where.Field("user_id").Eq(userID),
).Search("hello world")
```

## Repository Integration

Add a `Search` method that uses Den's built-in search and Burrow's pagination:

```go
// Search performs a full-text search across note titles and content.
func (r *Repository) Search(
    ctx context.Context,
    userID string,
    query string,
    pr burrow.PageRequest,
) ([]Note, burrow.PageResult, error) {
    query = strings.TrimSpace(query)
    if query == "" {
        return nil, burrow.PageResult{}, nil
    }

    // Count total matches for pagination.
    count, err := den.NewQuery[Note](ctx, r.db,
        where.Field("user_id").Eq(userID),
    ).Count()
    if err != nil {
        return nil, burrow.PageResult{}, fmt.Errorf("count search results: %w", err)
    }

    // Fetch matching notes ranked by relevance.
    results, err := den.NewQuery[Note](ctx, r.db,
        where.Field("user_id").Eq(userID),
    ).Limit(pr.Limit).Skip(pr.Offset()).Search(query)
    if err != nil {
        return nil, burrow.PageResult{}, fmt.Errorf("search notes: %w", err)
    }

    return results, burrow.OffsetResult(pr, count), nil
}
```

## Handling User Input

Full-text search supports query syntax that users can leverage for more precise searches:

| Query | Meaning |
|-------|---------|
| `hello world` | Rows containing both "hello" and "world" |
| `hello OR world` | Rows containing either word |
| `"hello world"` | Exact phrase match |
| `hello NOT world` | "hello" but not "world" |
| `hel*` | Prefix match — "hello", "help", etc. |

In most cases, you want to pass user input directly and let users benefit from this syntax. The main thing to handle is invalid queries — e.g., unmatched quotes or stray operators — which cause the search to return an error.

A straightforward approach: attempt the query and fall back to an empty result set on syntax errors.

```go
results, pageResult, err := repo.Search(ctx, userID, query, pr)
if err != nil {
    // Search syntax errors — return empty results instead of a 500 error.
    if strings.Contains(err.Error(), "syntax error") {
        results, pageResult = nil, burrow.PageResult{}
    } else {
        return err
    }
}
```

!!! important "Empty queries"
    Always check for empty/whitespace-only input before calling search. The `Search` method above already handles this by returning early when `query` is blank.

## ModelAdmin Integration

ModelAdmin auto-detects FTS-indexed fields at boot time. If a document type has fields with the `fts` tag option, ModelAdmin automatically uses full-text search instead of `LIKE` queries — no configuration needed.

This gives admin search all the benefits of full-text search: word-based matching, query syntax (AND, OR, NOT, prefix with `*`), and better performance on large datasets. If a user's search query has syntax errors (e.g., unmatched quotes), ModelAdmin falls back to `LIKE` transparently.

To enable search for your model's admin:

1. Add `den:"fts"` tags to your searchable fields
2. Set `SearchFields` on your `ModelAdmin` — that's it

```go
ma := &modeladmin.ModelAdmin[Note]{
    // ...
    SearchFields: []string{"title", "body"},
}
```

See the [Admin docs](../contrib/admin.md#fts5-auto-detection) for more details.

## Working Example

The [Notes example app](https://github.com/oliverandrich/burrow/tree/main/example/notes) implements full-text search end-to-end — document definitions with FTS tags, repository search method, and an HTMX-powered search form. Read its source to see all the pieces working together.
