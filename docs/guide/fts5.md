# Full-Text Search

SQLite ships with [FTS5](https://www.sqlite.org/fts5.html), a full-text search engine that supports word-based matching, relevance ranking, and match highlighting. Because Burrow uses `modernc.org/sqlite` (which compiles FTS5 by default), full-text search works out-of-the-box — no build tags or CGO required.

Compared to `LIKE '%term%'` queries, FTS5 offers:

| Feature | `LIKE` | FTS5 |
|---------|--------|------|
| Word-based matching | No (substring only) | Yes |
| Relevance ranking | No | Yes (`bm25()`) |
| Match highlighting | No | Yes (`snippet()`, `highlight()`) |
| Performance at scale | O(n) full scan | O(log n) inverted index |

## Creating the FTS5 Table

FTS5 uses [external content tables](https://www.sqlite.org/fts5.html#external_content_tables) to avoid duplicating data. The FTS index references your real table and stays in sync via triggers.

Using the notes app as an example, create a migration that adds an FTS5 index on the `title` and `content` columns:

```sql
-- 002_create_notes_fts.up.sql

-- FTS5 virtual table backed by the notes table.
-- content= links it as an external content table (no data duplication).
-- content_rowid= maps the FTS rowid to the notes primary key.
CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(
    title,
    content,
    content='notes',
    content_rowid='id',
    tokenize='unicode61'
);

-- Triggers to keep the FTS index in sync with the notes table.

CREATE TRIGGER IF NOT EXISTS notes_ai AFTER INSERT ON notes BEGIN
    INSERT INTO notes_fts(rowid, title, content)
    VALUES (new.id, new.title, new.content);
END;

CREATE TRIGGER IF NOT EXISTS notes_ad AFTER DELETE ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, title, content)
    VALUES ('delete', old.id, old.title, old.content);
END;

CREATE TRIGGER IF NOT EXISTS notes_au AFTER UPDATE ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, title, content)
    VALUES ('delete', old.id, old.title, old.content);
    INSERT INTO notes_fts(rowid, title, content)
    VALUES (new.id, new.title, new.content);
END;
```

!!! important "Tokenizer choice"
    `unicode61` handles most languages well. For English-only content, use `tokenize='porter unicode61'` to enable stemming (e.g., "running" matches "run"). See [FTS5 Tokenizers](https://www.sqlite.org/fts5.html#tokenizers) for all options.

## Repository Integration

Add a `Search` method that queries the FTS index and joins back to the content table for the full row data. Use offset-based pagination — cursor-based pagination is not reliable here because relevance ranks change as the index is updated.

```go
// SearchResult holds a note with an optional search snippet.
type SearchResult struct {
    Note
    Snippet string
}

// Search performs a full-text search across note titles and content.
func (r *Repository) Search(
    ctx context.Context,
    userID int64,
    query string,
    pr burrow.PageRequest,
) ([]SearchResult, burrow.PageResult, error) {
    query = strings.TrimSpace(query)
    if query == "" {
        return nil, burrow.PageResult{}, nil
    }

    // Count total matches for pagination.
    count, err := r.db.NewRaw(`
        SELECT COUNT(*)
        FROM notes_fts
        JOIN notes ON notes.id = notes_fts.rowid
        WHERE notes_fts MATCH ?
          AND notes.user_id = ?
          AND notes.deleted_at IS NULL`,
        query, userID,
    ).Scan(ctx, &count)
    if err != nil {
        return nil, burrow.PageResult{}, fmt.Errorf("count search results: %w", err)
    }

    // Fetch matching notes ranked by relevance.
    var results []SearchResult
    err = r.db.NewRaw(`
        SELECT
            notes.*,
            snippet(notes_fts, 1, '<mark>', '</mark>', '…', 32) AS snippet
        FROM notes_fts
        JOIN notes ON notes.id = notes_fts.rowid
        WHERE notes_fts MATCH ?
          AND notes.user_id = ?
          AND notes.deleted_at IS NULL
        ORDER BY bm25(notes_fts)
        LIMIT ? OFFSET ?`,
        query, userID, pr.Limit, (pr.Page-1)*pr.Limit,
    ).Scan(ctx, &results)
    if err != nil {
        return nil, burrow.PageResult{}, fmt.Errorf("search notes: %w", err)
    }

    return results, burrow.OffsetResult(pr, count), nil
}
```

!!! warning "Raw SQL required"
    Bun's query builder does not support FTS5 virtual tables directly. Use `db.NewRaw()` for FTS queries. The join back to the real `notes` table lets you use soft-delete filters and access all columns.

## Handling User Input

FTS5 supports a [query syntax](https://www.sqlite.org/fts5.html#full_text_query_syntax) that users can leverage for more precise searches:

| Query | Meaning |
|-------|---------|
| `hello world` | Rows containing both "hello" and "world" |
| `hello OR world` | Rows containing either word |
| `"hello world"` | Exact phrase match |
| `hello NOT world` | "hello" but not "world" |
| `hel*` | Prefix match — "hello", "help", etc. |

In most cases, you want to pass user input directly to FTS5 and let users benefit from this syntax. The main thing to handle is invalid queries — e.g., unmatched quotes or stray operators — which cause `MATCH` to return an error.

A straightforward approach: attempt the query and fall back to an empty result set on syntax errors.

```go
results, pageResult, err := repo.Search(ctx, userID, query, pr)
if err != nil {
    // FTS5 syntax errors surface as SQLite errors.
    // Return empty results instead of a 500 error.
    if strings.Contains(err.Error(), "fts5: syntax error") {
        results, pageResult = nil, burrow.PageResult{}
    } else {
        return err
    }
}
```

!!! important "Empty queries"
    Always check for empty/whitespace-only input before calling `MATCH`. An empty string is a syntax error in FTS5. The `Search` method above already handles this by returning early when `query` is blank.

## Highlighting Matches

FTS5 provides two functions for highlighting matches in results:

- **`highlight(table, col_index, before, after)`** — wraps every match in the given markers, returns the full column value
- **`snippet(table, col_index, before, after, ellipsis, max_tokens)`** — returns a short excerpt around the first match

The `col_index` is zero-based and refers to the column order in your `CREATE VIRTUAL TABLE` statement. In the notes example, `title` is column 0 and `content` is column 1.

```sql
-- Full title with matches wrapped in <mark> tags:
SELECT highlight(notes_fts, 0, '<mark>', '</mark>') AS title_hl
FROM notes_fts WHERE notes_fts MATCH ?;

-- Short content snippet (~32 tokens) with ellipsis:
SELECT snippet(notes_fts, 1, '<mark>', '</mark>', '…', 32) AS content_snippet
FROM notes_fts WHERE notes_fts MATCH ?;
```

To scan snippet columns into your Go structs, add a field with the `bun:"snippet"` tag (or use `bun.NullString` for nullable results):

```go
type SearchResult struct {
    Note
    Snippet string `bun:"snippet"`
}
```

!!! important "HTML safety"
    The `<mark>` tags in snippet output are raw HTML. When rendering in templates, use the `safeHTML` function or `template.HTML` to prevent double-escaping. Make sure the underlying text content is already safe (e.g., user input was sanitized on write).

## Performance Considerations

### External content tables

The `content='notes'` approach used above avoids duplicating data. The FTS index stores only the inverted index and metadata, referencing the original table for content. This is the best default for most use cases.

### Rebuilding the index

If triggers were added after data already existed, or if you suspect index corruption, rebuild with:

```sql
INSERT INTO notes_fts(notes_fts) VALUES('rebuild');
```

Run this in a migration or a one-off admin command. It re-reads every row from the content table.

### Content-less tables

For write-heavy workloads where you only need search ranking (not snippets or highlights), consider a [content-less table](https://www.sqlite.org/fts5.html#contentless_tables) by omitting the `content=` parameter:

```sql
CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(title, content);
```

Content-less tables are smaller and faster to write, but `snippet()` and `highlight()` are not available.

## ModelAdmin Integration

ModelAdmin auto-detects FTS5 tables at boot time. If a table named `{tablename}_fts` exists in `sqlite_master` (e.g., `notes_fts` for a model with `bun:"table:notes"`), ModelAdmin automatically switches from `LIKE` to FTS5 `MATCH` queries — no configuration needed.

This gives admin search all the benefits of FTS5: word-based matching, query syntax (AND, OR, NOT, prefix with `*`), and better performance on large datasets. If a user's search query has FTS5 syntax errors (e.g., unmatched quotes), ModelAdmin falls back to `LIKE` transparently.

To enable FTS5 for your model's admin search:

1. Create the FTS5 virtual table and triggers as shown [above](#creating-the-fts5-table)
2. Set `SearchFields` on your `ModelAdmin` — that's it

```go
ma := &modeladmin.ModelAdmin[Note]{
    // ...
    SearchFields: []string{"title", "body"},
}
```

See the [Admin docs](../contrib/admin.md#fts5-auto-detection) for more details.
