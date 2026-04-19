# Part 2: Database & Models

In this part you'll define the data models for your polls app, register them as Den documents, and create a repository for database access.

**Source code:** [`tutorial/step02/`](https://github.com/oliverandrich/burrow/tree/main/tutorial/step02)

## The Polls App

The polls app lives in its own package. Create the directory first:

```bash
mkdir -p internal/polls
```

All the code in this section — models, repository, and app setup — goes into `internal/polls/polls.go`. We'll split it into separate files as it grows.

### Models

Start `internal/polls/polls.go` with the package declaration and two models:

```go
package polls

import (
    "context"
    "time"

    "github.com/oliverandrich/burrow"
    "github.com/oliverandrich/den"
    "github.com/oliverandrich/den/document"
    "github.com/oliverandrich/den/where"
)

type Question struct {
    document.Base
    Text        string    `json:"text" den:"index"`
    PublishedAt time.Time `json:"published_at"`
}

type Choice struct {
    document.Base
    QuestionID string `json:"question_id" den:"index"`
    Text       string `json:"text"`
    Votes      int    `json:"votes"`
}
```

Key points:

- **`document.Base`** provides ULID-based ID, revision tracking, and timestamps
- **`den:"index"`** tags add secondary indexes for efficient queries
- **Relations** between questions and choices are managed via the `QuestionID` field — Den uses document references rather than ORM-style relation declarations

### Document Registration

Den handles schema creation automatically. Instead of writing SQL migration files, you register your document types in the app setup (see below). Den creates collections and indexes on startup.

!!! note "No down migrations"
    Den manages schema automatically. If you need to remove a field, simply remove it from your struct — existing documents retain their data but the field won't be read.

### Repository

Still in `internal/polls/polls.go`, add the repository below the models:

```go
type Repository struct {
    db *den.DB
}

func NewRepository(db *den.DB) *Repository {
    return &Repository{db: db}
}

func (r *Repository) ListQuestions(ctx context.Context) ([]Question, error) {
    return den.NewQuery[Question](r.db).
        Sort("published_at", den.Desc).
        All(ctx)
}

func (r *Repository) GetQuestion(ctx context.Context, id string) (*Question, error) {
    return den.FindByID[Question](ctx, r.db, id)
}

func (r *Repository) GetChoicesForQuestion(ctx context.Context, questionID string) ([]Choice, error) {
    return den.NewQuery[Choice](r.db,
        where.Field("question_id").Eq(questionID),
    ).All(ctx)
}
```

### App Setup

Still in `internal/polls/polls.go`, add the app struct:

```go
type App struct {
    repo *Repository
}

func New() *App { return &App{} }

func (a *App) Name() string { return "polls" }

func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
    a.repo = NewRepository(cfg.DB)
    return nil
}

func (a *App) Documents() []any {
    return []any{&Question{}, &Choice{}}
}
```

The app implements three interfaces:

| Interface | Method | Purpose |
|-----------|--------|---------|
| `burrow.App` | `Name()` | Required for all apps |
| `burrow.Configurable` | `Configure()` | App initialisation with database access |
| `burrow.HasDocuments` | `Documents()` | Automatic document collection setup |

### Update main.go

Add the polls app to the server:

```go
import "polls/internal/polls"

srv := burrow.NewServer(
    &homepageApp{},
    polls.New(),          // new
)
```

## Run It

After adding new imports, always run `go mod tidy` to fetch dependencies:

```bash
go mod tidy
go run .
```

When the server starts, Den automatically creates the `question` and `choice` collections in your SQLite database.

There are no routes yet for the polls app — we'll add those with templates in the next part.

## Next

In [Part 3](part3.md), you'll add HTML templates, a layout with Bootstrap styling, and views to list and display questions.
