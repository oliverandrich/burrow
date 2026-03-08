# Database

Burrow uses SQLite as its embedded database — no external database server required. The database file is created automatically on first startup.

## Why SQLite?

SQLite fits the "download, start, use" philosophy. There is no database server to install, configure, or maintain. Your entire application — code, templates, and data — lives in a single binary plus one database file. This makes deployment trivial, whether you're self-hosting on a VPS, running in Docker, or distributing an internal tool.

SQLite is an excellent choice for read-heavy workloads at any scale. But it also performs remarkably well for write-heavy applications with a limited number of concurrent users — which covers the majority of self-hosted apps, internal tools, and small-to-medium web applications. With WAL mode and the connection pool defaults that Burrow configures out of the box, you get solid concurrent read/write performance without any tuning.

Burrow uses [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite), a pure Go SQLite implementation. This means:

- **No CGO required** — builds with `CGO_ENABLED=0`, cross-compiles to any platform Go supports
- **No system dependencies** — no `libsqlite3-dev`, no shared libraries
- **Single binary** — everything is statically linked

## How It Works

At startup, Burrow opens the SQLite database and configures it with production-ready defaults inspired by [dj-lite](https://github.com/adamghill/dj-lite/):

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
    ./myapp --database-dsn ./data/myapp.db
    ```

=== "Environment Variable"

    ```bash
    DATABASE_DSN=./data/myapp.db ./myapp
    ```

=== "TOML Config"

    ```toml
    [database]
    dsn = "./data/myapp.db"
    ```

The default is `app.db` in the working directory. The parent directory must exist — Burrow creates the file but not the directory.

For testing, you can use an in-memory database:

```bash
./myapp --database-dsn ":memory:"
```

## Working with Bun

Burrow uses [Bun](https://bun.uptrace.dev/) as its ORM. Apps receive a `*bun.DB` instance via `AppConfig` during registration.

### Defining Models

Models are Go structs with `bun` struct tags:

```go
type Note struct {
    bun.BaseModel `bun:"table:notes,alias:n"`
    ID            int64     `bun:",pk,autoincrement"`
    UserID        int64     `bun:",notnull"`
    Title         string    `bun:",notnull"`
    Content       string    `bun:",notnull,default:''"`
    CreatedAt     time.Time `bun:",nullzero,notnull,default:current_timestamp"`
    DeletedAt     time.Time `bun:",soft_delete,nullzero"`
}
```

Common struct tags:

| Tag | Purpose |
|---|---|
| `bun:"table:name,alias:x"` | Table name and query alias |
| `bun:",pk,autoincrement"` | Primary key with auto-increment |
| `bun:",notnull"` | NOT NULL constraint |
| `bun:",unique"` | Unique constraint |
| `bun:",nullzero"` | Treat Go zero values as SQL NULL |
| `bun:",default:value"` | SQL default value |
| `bun:",soft_delete,nullzero"` | Soft delete — queries automatically filter deleted rows |
| `bun:"rel:has-many,join:id=user_id"` | One-to-many relationship |

### Queries

Bun provides a fluent query builder:

```go
// Select one
var note Note
err := db.NewSelect().Model(&note).Where("n.id = ?", id).Scan(ctx)

// Select many
var notes []Note
err := db.NewSelect().Model(&notes).
    Where("user_id = ?", userID).
    Order("created_at DESC").
    Scan(ctx)

// Insert
note := &Note{UserID: 1, Title: "Hello"}
_, err := db.NewInsert().Model(note).Exec(ctx)

// Update
_, err := db.NewUpdate().Model(note).WherePK().Exec(ctx)

// Partial update
_, err := db.NewUpdate().Model((*Note)(nil)).
    Set("title = ?", "New Title").
    Where("id = ?", id).
    Exec(ctx)

// Soft delete (sets deleted_at)
_, err := db.NewDelete().Model((*Note)(nil)).Where("id = ?", id).Exec(ctx)

// Hard delete (bypasses soft delete)
_, err := db.NewDelete().Model((*Note)(nil)).Where("id = ?", id).ForceDelete().Exec(ctx)

// Count
count, err := db.NewSelect().Model((*Note)(nil)).Where("user_id = ?", userID).Count(ctx)

// Exists check
exists, err := db.NewSelect().Model((*Note)(nil)).Where("id = ?", id).Exists(ctx)
```

### Relations

Load related records with `.Relation()`:

```go
type User struct {
    bun.BaseModel `bun:"table:users,alias:u"`
    ID            int64        `bun:",pk,autoincrement"`
    Username      string       `bun:",unique,notnull"`
    Credentials   []Credential `bun:"rel:has-many,join:id=user_id"`
}

// Eager-load credentials with the user
var user User
err := db.NewSelect().Model(&user).
    Relation("Credentials").
    Where("u.id = ?", id).
    Scan(ctx)
```

### Transactions

Use `db.BeginTx()` for atomic operations:

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback()

if _, err := tx.NewInsert().Model(note).Exec(ctx); err != nil {
    return err
}
if _, err := tx.NewInsert().Model(tag).Exec(ctx); err != nil {
    return err
}

return tx.Commit()
```

## Repository Pattern

Burrow's contrib apps use a repository pattern to encapsulate database access. This keeps handlers clean and makes testing easier.

```go
// Repository wraps database access for an app.
type Repository struct {
    db *bun.DB
}

func NewRepository(db *bun.DB) *Repository {
    return &Repository{db: db}
}

func (r *Repository) GetNoteByID(ctx context.Context, id int64) (*Note, error) {
    var note Note
    if err := r.db.NewSelect().Model(&note).Where("n.id = ?", id).Scan(ctx); err != nil {
        return nil, fmt.Errorf("get note %d: %w", id, err)
    }
    return &note, nil
}

func (r *Repository) CreateNote(ctx context.Context, note *Note) error {
    if _, err := r.db.NewInsert().Model(note).Exec(ctx); err != nil {
        return fmt.Errorf("create note: %w", err)
    }
    return nil
}
```

Wire it up in your app's `Register()` method:

```go
func (a *App) Register(cfg *burrow.AppConfig) error {
    a.repo = NewRepository(cfg.DB)
    return nil
}
```

Handlers then use the repository through the app:

```go
func (a *App) handleGetNote(w http.ResponseWriter, r *http.Request) error {
    id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
    note, err := a.repo.GetNoteByID(r.Context(), id)
    if err != nil {
        return burrow.NewHTTPError(http.StatusNotFound, "Note not found")
    }
    return burrow.JSON(w, http.StatusOK, note)
}
```

## Migrations

Each app manages its own SQL migrations. See the [Migrations](migrations.md) guide for full details on creating and managing migrations.

## Further Reading

- [Bun documentation](https://bun.uptrace.dev/) — full ORM reference
- [SQLite documentation](https://www.sqlite.org/docs.html) — SQL syntax and features
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — the pure Go SQLite driver
