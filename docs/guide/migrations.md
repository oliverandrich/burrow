# Migrations

The framework provides per-app SQL migrations tracked in a shared `_migrations` table.

## How It Works

1. Each app embeds its SQL files with `//go:embed migrations`
2. At startup, the framework calls `RunAppMigrations` for every app that implements `Migratable`
3. Migrations are applied in filename order, skipping already-applied ones
4. Each migration is namespaced by app name in the `_migrations` tracking table

## Creating Migrations

### 1. Create the SQL File

Place migration files in a `migrations/` directory inside your app package:

```
myapp/
├── myapp.go
└── migrations/
    ├── 001_create_things.up.sql
    └── 002_add_status_column.up.sql
```

### 2. Naming Convention

Files must end with `.up.sql`. Use a numeric prefix for ordering:

```
001_create_things.up.sql
002_add_status_column.up.sql
003_add_index_on_status.up.sql
```

The framework sorts filenames lexicographically and applies them in order.

### 3. Embed the Directory

```go
import "embed"

//go:embed migrations
var migrationFS embed.FS
```

### 4. Implement Migratable

```go
func (a *App) MigrationFS() fs.FS {
    sub, _ := fs.Sub(migrationFS, "migrations")
    return sub
}
```

!!! important
    Use `fs.Sub()` to strip the `migrations/` prefix. The framework expects the FS root to contain `.up.sql` files directly.

## Example Migration

```sql
-- 001_create_notes.up.sql
CREATE TABLE IF NOT EXISTS notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_notes_user_id ON notes (user_id);
```

## Tracking Table

The `_migrations` table is created automatically:

```sql
CREATE TABLE IF NOT EXISTS _migrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app TEXT NOT NULL,
    name TEXT NOT NULL,
    applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(app, name)
);
```

Each record stores the app name and migration filename. This namespacing means two apps can both have `001_initial.up.sql` without conflict.

## Migration Order

Migrations run in app registration order (the order you pass apps to `NewServer`), then by filename within each app. All migrations run before any app's `Register()` method is called.

## Tips

- Use `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS` for idempotent migrations
- Keep migrations small and focused — one table or one alteration per file
- Never modify an already-applied migration — create a new one instead
- The framework does not support down migrations — roll forward with new migrations
