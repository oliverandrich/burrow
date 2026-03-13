# Part 6: Admin Panel

In this part you'll add an admin panel with automatic CRUD for questions and choices using `ModelAdmin`.

**Source code:** [`tutorial/step06/`](https://github.com/oliverandrich/burrow/tree/main/tutorial/step06)

## Add the Admin App

The `admin` contrib app coordinates the admin panel. It provides a dashboard, layout, and route grouping. Apps contribute admin pages by implementing `HasAdmin`.

Update `main.go`:

```go
import (
    "github.com/oliverandrich/burrow/contrib/admin"
)

srv := burrow.NewServer(
    // ... existing apps ...
    polls.New(),
    admin.New(),          // new
)
```

The admin app:

- Provides routes under `/admin/` protected by `auth.RequireAdmin()` middleware
- Collects nav items and routes from all `HasAdmin` apps
- Applies its own layout with a sidebar navigation

## Prepare the Models

In `internal/polls/polls.go`, add `verbose` struct tags to both models so ModelAdmin knows how to label columns. Also add `form:"-"` to the `Choices` relation on `Question` and the `Question` relation on `Choice` — ModelAdmin cannot handle nested relations in forms, so we exclude them:

```go
type Question struct {
    bun.BaseModel `bun:"table:questions,alias:q"`

    ID          int64     `bun:",pk,autoincrement" verbose:"ID"`
    Text        string    `bun:",notnull" verbose:"Question"`
    PublishedAt time.Time `bun:",notnull,default:current_timestamp" verbose:"Published"`

    Choices []Choice `bun:"rel:has-many,join:id=question_id" form:"-"`
}

// String returns the question text for display in admin list views (e.g. as FK label).
func (q Question) String() string { return q.Text }

type Choice struct {
    bun.BaseModel `bun:"table:choices,alias:c"`

    ID         int64  `bun:",pk,autoincrement" verbose:"ID"`
    QuestionID int64  `bun:",notnull" verbose:"Question"`
    Text       string `bun:",notnull" verbose:"Choice"`
    Votes      int    `bun:",notnull,default:0" verbose:"Votes"`

    Question *Question `bun:"rel:belongs-to,join:question_id=id" form:"-" verbose:"Question"`
}
```

The `String()` method on `Question` tells ModelAdmin how to display a question when it appears as a foreign key in list views. Any type that implements `fmt.Stringer` is rendered using its `String()` result instead of the raw struct.

## Set Up ModelAdmin

`ModelAdmin` provides generic CRUD views for any Bun model. In `internal/polls/polls.go`, add the imports and update the `App` struct:

```go
import (
    "strconv"

    "github.com/oliverandrich/burrow/contrib/admin/modeladmin"
    matpl "github.com/oliverandrich/burrow/contrib/admin/modeladmin/templates"
)

type App struct {
    repo           *Repository
    handlers       *Handlers
    questionsAdmin *modeladmin.ModelAdmin[Question]
    choicesAdmin   *modeladmin.ModelAdmin[Choice]
}
```

Update the `Register()` method in `internal/polls/polls.go` to initialise both ModelAdmins:

```go
func (a *App) Register(cfg *burrow.AppConfig) error {
    a.repo = NewRepository(cfg.DB)
    a.handlers = &Handlers{repo: a.repo}

    a.questionsAdmin = &modeladmin.ModelAdmin[Question]{
        Slug:              "questions",
        DisplayName:       "Question",
        DisplayPluralName: "Questions",
        DB:                cfg.DB,
        Renderer:          matpl.DefaultRenderer[Question](),
        CanCreate:         true,
        CanEdit:           true,
        CanDelete:         true,
        ListFields:        []string{"ID", "Text", "PublishedAt"},
        OrderBy:           "published_at DESC, id DESC",
    }

    a.choicesAdmin = &modeladmin.ModelAdmin[Choice]{
        Slug:              "choices",
        DisplayName:       "Choice",
        DisplayPluralName: "Choices",
        DB:                cfg.DB,
        Renderer:          matpl.DefaultRenderer[Choice](),
        CanCreate:         true,
        CanEdit:           true,
        CanDelete:         true,
        ListFields:        []string{"ID", "Question", "Text", "Votes"},
        Relations:         []string{"Question"},
        OrderBy:           "c.question_id, c.id",
        FieldChoices: map[string]modeladmin.ChoicesFunc{
            "QuestionID": func(ctx context.Context) ([]modeladmin.Choice, error) {
                var questions []Question
                err := cfg.DB.NewSelect().Model(&questions).
                    Order("published_at DESC").Scan(ctx)
                if err != nil {
                    return nil, err
                }
                choices := make([]modeladmin.Choice, len(questions))
                for i, q := range questions {
                    choices[i] = modeladmin.Choice{
                        Value: strconv.FormatInt(q.ID, 10),
                        Label: q.Text,
                    }
                }
                return choices, nil
            },
        },
    }
    return nil
}
```

The `FieldChoices` map tells ModelAdmin to render `QuestionID` as a `<select>` dropdown instead of a plain number input. The function loads all questions from the database at request time, so new questions appear automatically.

## Implement HasAdmin

Still in `internal/polls/polls.go`, implement the `HasAdmin` interface with its two methods `AdminRoutes()` and `AdminNavItems()`:

```go
func (a *App) AdminNavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {Label: "Questions", URL: "/admin/questions", Position: 30},
        {Label: "Choices", URL: "/admin/choices", Position: 31},
    }
}

func (a *App) AdminRoutes(r chi.Router) {
    a.questionsAdmin.Routes(r)
    a.choicesAdmin.Routes(r)
}
```

`AdminRoutes` receives a router scoped to `/admin/`. Each `ModelAdmin.Routes()` call mounts list, create, detail, update, and delete routes under its slug, so the full paths become `/admin/questions` and `/admin/choices`.

## Add an Admin Link to the Navbar

The admin panel has its own sidebar navigation, but users need a way to get there. In `internal/pages/pages.go`, add an admin NavItem:

```go
func (a *App) NavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {Label: "Home", URL: "/", Position: 0},
        {Label: "Admin", URL: "/admin", Position: 100, AdminOnly: true},
    }
}
```

Items with `AdminOnly: true` are automatically hidden from non-admin users. The `navLinks` template function handles the filtering — the `auth` middleware injects an `AuthChecker` into the context, and `navLinks` reads it to decide which items to show. No manual filtering code needed.

## Run It

```bash
go mod tidy
go run .
```

Register a user, then promote them to admin via the database:

```bash
sqlite3 app.db "UPDATE users SET role = 'admin' WHERE id = 1"
```

Visit `/admin/` to see the dashboard. Click "Questions" in the sidebar to create a question, then click "Choices" to add choices for it — the question dropdown shows all available questions.

## What You've Learnt

- **`admin.New()`** — coordinates the admin panel with built-in default layout and dashboard
- **`ModelAdmin`** — generic CRUD views for any Bun model, configured declaratively
- **`fmt.Stringer` for FK labels** — implement `String()` on related models to display human-readable labels instead of raw IDs in list views
- **`Relations`** — eager-load Bun relations so list views can display related model data
- **`FieldChoices`** — dynamic select dropdowns for foreign key fields in forms, loaded from the database at request time
- **`HasAdmin`** — interface for apps to contribute admin routes and navigation
- **`verbose` struct tags** — provide human-readable column labels for the admin UI
- **`form:"-"`** — excludes fields (like relations) from the admin form

## Next

In [Part 7](part7.md), you'll add HTMX for smooth navigation and infinite scroll pagination.
