# Database

Burrow uses [Den](https://github.com/oliverandrich/den), an object-document mapper (ODM) for Go. Den supports two storage backends — choose the one that fits your deployment:

## SQLite (Default)

SQLite fits the "download, start, use" philosophy. No database server to install or maintain. Your application compiles to a single binary, and the data lives in one file next to it. Ideal for self-hosted apps, internal tools, and prototyping.

- **No CGO required** — pure Go via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite), cross-compiles anywhere
- **Zero dependencies** — no `libsqlite3-dev`, no shared libraries
- **Production-ready** — WAL mode, connection pooling, and tuned PRAGMAs out of the box

## PostgreSQL

When you need replication, concurrent writers, or a managed database service, switch to PostgreSQL — no code changes required. Same Den API, same document types, different backend.

- **Full JSONB support** — GIN indexes for fast queries
- **Multi-writer concurrency** — no single-writer bottleneck
- **Managed hosting** — works with any PostgreSQL provider

## How It Works

At startup, Burrow opens the database using the DSN (Data Source Name — a URL-style connection string, e.g., `sqlite:///app.db` or `postgres://user:pass@host/db`) and configures it with production-ready defaults inspired by [dj-lite](https://github.com/adamghill/dj-lite/). For SQLite, per-connection PRAGMAs are set via `_pragma` DSN parameters so they apply to every connection in the pool:

| PRAGMA | Value | Purpose |
|---|---|---|
| `journal_mode` | `WAL` | Write-Ahead Logging for concurrent reads during writes |
| `synchronous` | `NORMAL` | Balance between durability and write performance |
| `foreign_keys` | `ON` | Enforce referential integrity |
| `busy_timeout` | `5000` (5s) | Wait up to 5 seconds for a lock instead of failing immediately |
| `temp_store` | `MEMORY` | Store temporary tables in RAM for faster queries |
| `mmap_size` | `134217728` (128MB) | Memory-mapped I/O for improved read performance |
| `journal_size_limit` | `27103364` (~26MB) | Prevent WAL journal files from growing indefinitely |
| `cache_size` | `2000` | Number of database pages held in memory |

Additionally, Burrow sets the transaction mode to `IMMEDIATE`. This ensures that write transactions acquire a lock immediately and wait up to `busy_timeout` instead of failing with a "database is locked" error.

These settings are fixed and cannot be overridden. They are tuned for the typical Burrow use case — self-hosted applications with moderate concurrency — and should work well without any tuning.

The connection pool is configured with:

- Max 10 open connections
- Max 5 idle connections
- 1 hour connection lifetime

## Configuration

The database path is configured via the `--database-dsn` flag:

=== "CLI Flag"

    ```bash
    ./myapp --database-dsn sqlite:///data/myapp.db
    ```

=== "Environment Variable"

    ```bash
    DATABASE_DSN=sqlite:///data/myapp.db ./myapp
    ```

=== "TOML Config"

    ```toml
    [database]
    dsn = "sqlite:///data/myapp.db"
    ```

The default is `sqlite:///app.db` in the working directory. The parent directory must exist — Burrow creates the file but not the directory. For PostgreSQL, use a URL like `postgres://user:pass@localhost/mydb`.

For testing, you can use an in-memory SQLite database:

```bash
./myapp --database-dsn sqlite:///:memory:
```

## Working with Den

Burrow uses [Den](https://github.com/oliverandrich/den), an object-document mapper (ODM) for Go. Apps receive a `*den.DB` instance via `AppConfig` during registration. Den stores documents as JSON internally and uses ULID-based IDs via `document.Base`.

### Defining Documents

Documents are Go structs that embed `document.Base` for ID and timestamp management:

```go
type Note struct {
    document.Base
    UserID    string    `json:"user_id"`
    Title     string    `json:"title" den:"index"`
    Content   string    `json:"content"`
    CreatedAt time.Time `json:"created_at"`
}
```

Common struct tags:

| Tag | Purpose |
|---|---|
| `json:"name"` | JSON field name for serialization |
| `den:"index"` | Add a secondary index on this field |
| `den:"unique"` | Unique constraint on this field |
| `den:"fts"` | Full-text search index on this field |
| `den:"omitempty"` | Omit from JSON when empty |

### Queries

Den provides a chainable QuerySet API for queries and a functional API for mutations:

```go
// Find by ID
note, err := den.FindByID[Note](ctx, db, id)

// Find one with conditions
note, err := den.NewQuery[Note](ctx, db, where.Field("user_id").Eq(userID)).First()

// Find many with sorting
notes, err := den.NewQuery[Note](ctx, db,
    where.Field("user_id").Eq(userID),
).Sort("created_at", den.Desc).All()

// Insert
note := &Note{UserID: "01J...", Title: "Hello"}
err := den.Insert(ctx, db, note)

// Update (replace entire document)
err := den.Replace(ctx, db, note)

// Save (insert or update based on ID)
err := den.Save(ctx, db, note)

// Delete
err := den.Delete(ctx, db, note)

// Count
count, err := den.NewQuery[Note](ctx, db, where.Field("user_id").Eq(userID)).Count()

// Exists check
exists, err := den.NewQuery[Note](ctx, db, where.Field("id").Eq(id)).Exists()
```

### Transactions

Use `den.RunInTransaction()` for atomic operations:

```go
err := den.RunInTransaction(ctx, db, func(tx *den.Tx) error {
    if err := den.Insert(ctx, tx, note); err != nil {
        return err
    }
    if err := den.Insert(ctx, tx, tag); err != nil {
        return err
    }
    return nil
})
```

## Repository Pattern

Burrow's contrib apps use a repository pattern to encapsulate database access. This keeps handlers clean and makes testing easier.

```go
// Repository wraps database access for an app.
type Repository struct {
    db *den.DB
}

func NewRepository(db *den.DB) *Repository {
    return &Repository{db: db}
}

func (r *Repository) GetNoteByID(ctx context.Context, id string) (*Note, error) {
    note, err := den.FindByID[Note](ctx, r.db, id)
    if err != nil {
        return nil, fmt.Errorf("get note %s: %w", id, err)
    }
    return note, nil
}

func (r *Repository) CreateNote(ctx context.Context, note *Note) error {
    if err := den.Insert(ctx, r.db, note); err != nil {
        return fmt.Errorf("create note: %w", err)
    }
    return nil
}
```

Wire it up in your app's `Configure()` method:

```go
func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
    a.repo = NewRepository(cfg.DB)
    return nil
}
```

Handlers then use the repository through the app:

```go
func (a *App) handleGetNote(w http.ResponseWriter, r *http.Request) error {
    id := chi.URLParam(r, "id")
    note, err := a.repo.GetNoteByID(r.Context(), id)
    if err != nil {
        return burrow.NewHTTPError(http.StatusNotFound, "Note not found")
    }
    return burrow.JSON(w, http.StatusOK, note)
}
```

## Document Schema

Den automatically manages collections (tables) based on your document structs. When you register documents with your app, Den creates and updates the underlying schema — no manual SQL migrations needed for document structure.

See the [Migrations](migrations.md) guide for details on how schema management works with Den.

## Further Reading

- [Full-Text Search](fts5.md) — add full-text search to your app
- [Den documentation](https://github.com/oliverandrich/den) — full ODM reference
- [SQLite documentation](https://www.sqlite.org/docs.html) — SQL syntax and features
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — the pure Go SQLite driver
