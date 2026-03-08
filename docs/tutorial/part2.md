# Part 2: Database & Models

In this part you'll define the data models for your polls app, write a SQL migration, and create a repository for database access.

**Source code:** [`tutorial/step02/`](https://codeberg.org/oliverandrich/burrow/src/branch/main/tutorial/step02)

## The Polls App

Create `internal/polls/polls.go`. This single file will contain models, repository, and app setup — we'll split it into separate files as it grows.

### Models

Define two models: `Question` and `Choice`.

```go
package polls

import (
    "time"
    "github.com/uptrace/bun"
)

type Question struct {
    bun.BaseModel `bun:"table:questions,alias:q"`

    ID          int64     `bun:",pk,autoincrement"`
    Text        string    `bun:",notnull"`
    PublishedAt time.Time `bun:",notnull,default:current_timestamp"`

    Choices []Choice `bun:"rel:has-many,join:id=question_id"`
}

type Choice struct {
    bun.BaseModel `bun:"table:choices,alias:c"`

    ID         int64  `bun:",pk,autoincrement"`
    QuestionID int64  `bun:",notnull"`
    Text       string `bun:",notnull"`
    Votes      int    `bun:",notnull,default:0"`

    Question *Question `bun:"rel:belongs-to,join:question_id=id"`
}
```

Key points:

- **`bun.BaseModel`** with the `bun:"table:..."` tag maps the struct to a database table
- **`alias:q`** gives the table a short alias for use in queries (`q.id` instead of `questions.id`)
- **Relations** are declared with `rel:has-many` and `rel:belongs-to` — Bun uses these for eager loading

### Migration

Create `internal/polls/migrations/001_create_polls.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS questions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    text TEXT NOT NULL,
    published_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS choices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    votes INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_choices_question_id ON choices (question_id);
```

Burrow runs migrations automatically at startup for apps that implement `Migratable`. Migrations are tracked per-app in the `_migrations` table — each file runs exactly once.

### Repository

The repository encapsulates all database queries:

```go
type Repository struct {
    db *bun.DB
}

func NewRepository(db *bun.DB) *Repository {
    return &Repository{db: db}
}

func (r *Repository) ListQuestions(ctx context.Context) ([]Question, error) {
    var questions []Question
    err := r.db.NewSelect().
        Model(&questions).
        Order("published_at DESC", "id DESC").
        Scan(ctx)
    return questions, err
}

func (r *Repository) GetQuestion(ctx context.Context, id int64) (*Question, error) {
    question := new(Question)
    err := r.db.NewSelect().
        Model(question).
        Relation("Choices").
        Where("q.id = ?", id).
        Scan(ctx)
    if err != nil {
        return nil, err
    }
    return question, nil
}
```

Note how `Relation("Choices")` eagerly loads all choices for a question in a single query.

### App Setup

Wire everything together with the `App` struct:

```go
//go:embed migrations
var migrationFS embed.FS

type App struct {
    repo *Repository
}

func New() *App { return &App{} }

func (a *App) Name() string { return "polls" }

func (a *App) Register(cfg *burrow.AppConfig) error {
    a.repo = NewRepository(cfg.DB)
    return nil
}

func (a *App) MigrationFS() fs.FS { return migrationFS }
```

The app implements two interfaces:

| Interface | Method | Purpose |
|-----------|--------|---------|
| `burrow.App` | `Name()`, `Register()` | Required for all apps |
| `burrow.Migratable` | `MigrationFS()` | Automatic database migrations |

### Update main.go

Add the polls app to the server:

```go
srv := burrow.NewServer(
    session.New(),
    healthcheck.New(),
    polls.New(),
    &homepageApp{},
)
```

## Run It

```bash
go run .
```

When the server starts, you'll see a log line confirming the migration ran. The `questions` and `choices` tables now exist in your SQLite database.

There are no routes yet for the polls app — we'll add those with templates in the next part.

## Next

In [Part 3](part3.md), you'll add HTML templates, a layout with Bootstrap styling, and views to list and display questions.
