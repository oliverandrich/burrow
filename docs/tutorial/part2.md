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
    "fmt"
    "time"

    "github.com/oliverandrich/burrow"
    "github.com/oliverandrich/den"
    "github.com/oliverandrich/den/document"
    "github.com/oliverandrich/den/where"
    "github.com/urfave/cli/v3"
)

type Question struct {
    document.Base
    PublishedAt time.Time `json:"published_at" den:"index"`
    Text        string    `json:"text"`
    Choices     []Choice  `json:"-"`
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
- **`den:"index"`** tags add secondary indexes for efficient queries — `PublishedAt` is indexed because we sort the question list by it
- **`Choices []Choice` with `json:"-"`** is a regular Go slice that we populate manually from a follow-up `Choice` query (see `GetQuestion` below); the `json:"-"` tag keeps it out of Den's persisted JSON. There is no ORM-style relation declaration — Den uses document references via the `QuestionID` field

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
    ptrs, err := den.NewQuery[Question](r.db).
        Sort("published_at", den.Desc).
        Sort("_id", den.Desc).
        All(ctx)
    if err != nil {
        return nil, fmt.Errorf("list questions: %w", err)
    }
    questions := make([]Question, len(ptrs))
    for i, p := range ptrs {
        questions[i] = *p
    }
    return questions, nil
}

func (r *Repository) GetQuestion(ctx context.Context, id string) (*Question, error) {
    question, err := den.FindByID[Question](ctx, r.db, id)
    if err != nil {
        return nil, fmt.Errorf("get question %s: %w", id, err)
    }
    choicePtrs, err := den.NewQuery[Choice](r.db, where.Field("question_id").Eq(id)).All(ctx)
    if err != nil {
        return nil, fmt.Errorf("get choices for question %s: %w", id, err)
    }
    choices := make([]Choice, len(choicePtrs))
    for i, p := range choicePtrs {
        choices[i] = *p
    }
    question.Choices = choices
    return question, nil
}
```

Notes:

- `den.NewQuery[T](...).All(ctx)` returns `[]*T`; the small conversion loop copies pointers into value slices so handlers can pass `Question` and `Choice` by value into templates without worrying about nil dereferences.
- `ListQuestions` adds a secondary sort on `_id` (Den's internal ULID-based primary key) so questions with identical `PublishedAt` get a stable, newest-first order.
- `GetQuestion` folds choices into the parent question. Den doesn't auto-load relations — you decide where the boundary lives, which keeps query patterns explicit.

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

func (a *App) Documents() []document.Document {
    return []document.Document{&Question{}, &Choice{}}
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

In [Part 3](part3.md), you'll add HTML templates, a layout backed by a small hand-written stylesheet, and views to list and display questions.
