# Migrations

Den handles schema management automatically based on your document struct definitions. There are no hand-written SQL migration files.

## How It Works

1. Each app declares its document types by implementing `HasDocuments`
2. At startup, the framework calls `den.Register()` for every document type returned by each app
3. Den inspects the struct tags (`den:"index"`, `den:"unique"`, `den:"fts"`) and creates or updates the underlying collections and indexes automatically
4. Schema changes are applied idempotently — adding new fields or indexes is safe

## Implementing HasDocuments

```go
func (a *App) Documents() []any {
    return []any{
        &Note{},
        &Tag{},
    }
}
```

!!! important
    Return pointers to zero-value instances of each document type. Den uses these to introspect the struct and set up the collection schema.

## Document Definitions

Den derives the collection name from the struct name (lowercased, no pluralization). Override with `CollectionName` in `DenSettings()`:

```go
type Note struct {
    document.Base
    UserID  string `json:"user_id"`
    Title   string `json:"title" den:"index"`
    Content string `json:"content" den:"fts"`
    Status  string `json:"status" den:"index"`
}
```

This creates a `note` collection with indexes on `title` and `status`, and a full-text search index on `content`.

## Schema Evolution

Den handles additive schema changes automatically:

- **New fields** — added to existing documents as they are saved
- **New indexes** — created on the next startup
- **New FTS indexes** — created on the next startup

Removing fields or indexes requires no special handling — unused indexes are simply ignored.

## Data Migrations

Schema changes (new fields, new indexes) are handled automatically. But sometimes you need to transform existing data — rename a field, split a value, backfill a computed field. For these cases, Den provides a migration registry.

### Defining Migrations

Create a migration registry in your app and register versioned migration functions:

```go
var migrations = migrate.NewRegistry()

func init() {
    migrations.Register("001_backfill_slug", migrate.Migration{
        Forward: func(ctx context.Context, tx *den.Tx) error {
            for note, err := range den.NewQuery[Note](tx).Iter(ctx) {
                if err != nil {
                    return err
                }
                if note.Slug == "" {
                    note.Slug = slugify(note.Title)
                    if err := den.Update(ctx, tx, note); err != nil {
                        return err
                    }
                }
            }
            return nil
        },
    })
}
```

### Running Migrations

Run migrations during your app's `Configure()` phase:

```go
func (a *App) Configure(cfg *burrow.AppConfig, cmd *cli.Command) error {
    a.repo = NewRepository(cfg.DB)

    // Run data migrations (idempotent — tracks applied versions)
    if err := migrations.Up(context.Background(), cfg.DB); err != nil {
        return fmt.Errorf("notes: run migrations: %w", err)
    }

    return nil
}
```

### Key Points

- Each migration runs atomically in a transaction — if it fails, nothing is applied
- Applied migrations are tracked in a `_den_migrations` collection — running `Up()` again skips already-applied migrations
- `Forward` receives a `*den.Tx` — pass it to `den.Insert`, `den.Update`, `den.Delete` etc. for transactional safety (since den v0.8.0, the unified `Scope` interface accepts both `*den.DB` and `*den.Tx`)
- `Backward` is optional — define it if you need rollback support via `migrations.Down()`
- Migrations run **after** `den.Register()` has created the schema, so your document types are always available

## Migration Order

Documents are registered in app registration order (the order you pass apps to `NewServer`). All document schemas are set up before any app's `Configure()` method is called.

## Tips

- Keep document structs focused — one struct per logical entity
- Use `den:"index"` for fields you frequently query on
- Use `den:"unique"` for fields that must be unique across the collection
- Use `den:"fts"` for fields that need full-text search
