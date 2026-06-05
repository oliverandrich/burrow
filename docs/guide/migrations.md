# Database Migrations

Den handles schema management automatically based on your document struct definitions. There are no hand-written SQL migration files. For framework-version upgrades (e.g. v0.18 → v0.20) see the [Migration](../migration/v0.20.md) section instead.

## How It Works

1. Each app declares its document types by implementing `HasDocuments`
2. At startup, the framework calls `den.Register()` for every document type returned by each app
3. Den inspects the struct tags (`den:"index"`, `den:"unique"`, `den:"fts"`) and creates or updates the underlying collections and indexes automatically
4. Schema changes are applied idempotently — adding new fields or indexes is safe

## Implementing HasDocuments

```go
func (a *App) Documents() []document.Document {
    return []document.Document{
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
- **New FTS indexes** — created on the next startup, backfilled with existing rows

Removing fields or indexes requires no special handling — unused indexes are simply ignored.

## Data Migrations

Schema changes (new fields, new indexes) are handled automatically. But sometimes you need to seed initial data, transform existing data — rename a field, split a value, backfill a computed field. For these cases, apps implement the `HasMigrations` interface and the server applies them at boot via Den's migrate package.

### Implementing HasMigrations

Return one or more `burrow.NamedMigration` values from `Migrations()`. The framework registers them under `{app.Name()}/{version}` (so two contribs can both ship `"001_initial"` without colliding) and runs `migrate.Up` once after every app's `Configure()` has completed.

```go
import (
    "github.com/oliverandrich/burrow"
    "github.com/oliverandrich/den"
    "github.com/oliverandrich/den/migrate"
    "github.com/oliverandrich/den/where"
)

// Implements [burrow.HasMigrations].
func (a *App) Migrations() []burrow.NamedMigration {
    return []burrow.NamedMigration{{
        Version: "001_backfill_slug",
        Migration: migrate.Migration{
            Forward: func(ctx context.Context, tx *den.Tx) error {
                notes, err := den.NewQuery[Note](tx,
                    where.Field("slug").Eq(""),
                ).All(ctx)
                if err != nil {
                    return err
                }
                for _, note := range notes {
                    note.Slug = slugify(note.Title)
                    if err := den.Save(ctx, tx, note); err != nil {
                        return err
                    }
                }
                return nil
            },
        },
    }}
}
```

Forward functions use Den primitives (`den.Save`, `den.Delete`, `den.NewQuery`) directly with the transaction. Your app's `Repository` holds a `*den.DB` for the normal request path, but `Forward` receives a `*den.Tx` — calling `a.repo.Save(...)` would write outside the migration's transaction. The Den helpers polymorphically accept either type via an internal `Scope` interface, so passing the `*den.Tx` straight in is the correct path. Conceptually it also fits: migrations are setup code, not application code, and operating at the framework-persistence layer keeps the transaction boundary explicit.

### Seeding Initial Data

Seeding — inserting reference data, demo content, default lookup tables, an initial admin user — is just a forward-only migration. Same mechanism, no separate "seed" concept. The advantage over an ad-hoc seed script: the migration log records that the seed has run, so booting the app twice doesn't duplicate the rows.

```go
func (a *App) Migrations() []burrow.NamedMigration {
    return []burrow.NamedMigration{{
        Version: "001_initial_polls",
        Migration: migrate.Migration{
            Forward: func(ctx context.Context, tx *den.Tx) error {
                samples := []struct {
                    text    string
                    choices []string
                }{
                    {"What's your favourite Go web framework?", []string{"Burrow", "Gin", "Echo", "net/http alone"}},
                    {"How long have you been writing Go?", []string{"<1 year", "1–3 years", "3–5 years", "5+ years"}},
                }
                for _, s := range samples {
                    q := &Question{Text: s.text, PublishedAt: time.Now()}
                    if err := den.Save(ctx, tx, q); err != nil {
                        return err
                    }
                    for _, ct := range s.choices {
                        if err := den.Save(ctx, tx, &Choice{QuestionID: q.ID, Text: ct}); err != nil {
                            return err
                        }
                    }
                }
                return nil
            },
        },
    }}
}
```

The example doesn't guard against duplicates (`Exists()` check, etc.) because the `_den_migrations` log already guarantees the version runs exactly once. If you ever change a seed migration's body without bumping the version, that change won't apply to apps that already ran the old version — in that case create a follow-up migration (`002_extra_polls`) rather than editing `001_initial_polls` in place.

Re-seeding for local development: drop the database (`rm data/app.db`) and boot again. The fresh `_den_migrations` collection treats the seed as pending and re-runs it. There is no `--seed`-style command-line escape hatch — that's intentional; the version log is the single source of truth. If you need to keep some data (e.g. your dev user account) but re-run a specific migration, delete that row from `_den_migrations` manually and reboot.

### Key Points

- Each migration runs atomically in a transaction — if it fails, nothing is applied
- Applied migrations are tracked in a `_den_migrations` collection. Re-runs skip already-applied versions, so booting twice is a no-op
- `Forward` is required; `Backward` is optional. Without `Backward`, the migration is forward-only — `migrate.Down` / `DownOne` return an error if asked to roll back
- Migrations run **after** `den.Register()` has created the schema AND **after** every app's `Configure()` has wired its repo, so document types and app state are both ready
- Within an app, migrations run in the order returned from `Migrations()`. The version string is the lexicographic key the framework registers against `_den_migrations`, so the `001_`, `002_` prefix convention keeps the slice order and the registry order aligned
- A failed migration aborts boot: the server exits with a non-zero status and logs `migration {app}/{version} failed: {error}`. Subsequent migrations are skipped — fix the cause, redeploy, and the failed version retries on the next boot

### Running Migrations Manually

You don't need to call anything; the framework wires `migrate.Registry` from every `HasMigrations` app and runs `Up` during boot. If you need finer control (e.g. a custom `migrate` subcommand that calls `Down`), reach for Den's [migrate package](https://pkg.go.dev/github.com/oliverandrich/den/migrate) directly.

## Migration Order

Documents are registered in app registration order (the order you pass apps to `NewServer`). All document schemas are set up before any app's `Configure()` method is called.

## Tips

- Keep document structs focused — one struct per logical entity
- Use `den:"index"` for fields you frequently query on
- Use `den:"unique"` for fields that must be unique across the collection
- Use `den:"fts"` for fields that need full-text search
