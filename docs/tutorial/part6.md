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

In `internal/polls/polls.go`, add `verbose` struct tags to both models so ModelAdmin knows how to label columns. Also add `form:"-"` to the `Choices` relation on `Question` — ModelAdmin cannot handle nested relations, so we exclude it from the form:

```go
type Question struct {
    bun.BaseModel `bun:"table:questions,alias:q"`

    ID          int64     `bun:",pk,autoincrement" verbose:"ID"`
    Text        string    `bun:",notnull" verbose:"Question"`
    PublishedAt time.Time `bun:",notnull,default:current_timestamp" verbose:"Published"`

    Choices []Choice `bun:"rel:has-many,join:id=question_id" form:"-"`
}

type Choice struct {
    bun.BaseModel `bun:"table:choices,alias:c"`

    ID         int64  `bun:",pk,autoincrement" verbose:"ID"`
    QuestionID int64  `bun:",notnull" verbose:"Question"`
    Text       string `bun:",notnull" verbose:"Choice"`
    Votes      int    `bun:",notnull,default:0" verbose:"Votes"`

    Question *Question `bun:"rel:belongs-to,join:question_id=id" form:"-"`
}
```

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
        ListFields:        []string{"ID", "QuestionID", "Text", "Votes"},
        OrderBy:           "question_id, id",
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
    r.Route("/questions", func(r chi.Router) {
        r.Get("/", burrow.Handle(a.questionsAdmin.HandleList))
        r.Get("/new", burrow.Handle(a.questionsAdmin.HandleNew))
        r.Post("/new", burrow.Handle(a.questionsAdmin.HandleNew))
        r.Get("/{id}", burrow.Handle(a.questionsAdmin.HandleDetail))
        r.Post("/{id}", burrow.Handle(a.questionsAdmin.HandleDetail))
        r.Get("/{id}/delete", burrow.Handle(a.questionsAdmin.HandleDelete))
        r.Post("/{id}/delete", burrow.Handle(a.questionsAdmin.HandleDelete))
    })
    r.Route("/choices", func(r chi.Router) {
        r.Get("/", burrow.Handle(a.choicesAdmin.HandleList))
        r.Get("/new", burrow.Handle(a.choicesAdmin.HandleNew))
        r.Post("/new", burrow.Handle(a.choicesAdmin.HandleNew))
        r.Get("/{id}", burrow.Handle(a.choicesAdmin.HandleDetail))
        r.Post("/{id}", burrow.Handle(a.choicesAdmin.HandleDetail))
        r.Get("/{id}/delete", burrow.Handle(a.choicesAdmin.HandleDelete))
        r.Post("/{id}/delete", burrow.Handle(a.choicesAdmin.HandleDelete))
    })
}
```

`AdminRoutes` receives a router scoped to `/admin/`, so the full paths become `/admin/questions` and `/admin/choices`.

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

Items with `AdminOnly: true` are only visible to admin users. To make this work, your layout function needs to filter nav items based on the current user. Update the `Layout()` function in `internal/pages/pages.go`:

```go
layoutData := map[string]any{
    "Content":  content,
    "NavItems": visibleNavItems(r.Context()),
    "Messages": messages.Get(r.Context()),
    "User":     auth.UserFromContext(r.Context()),
}
```

And add a helper function that filters nav items:

```go
func visibleNavItems(ctx context.Context) []burrow.NavItem {
    user := auth.UserFromContext(ctx)
    var items []burrow.NavItem
    for _, item := range burrow.NavItems(ctx) {
        if item.AdminOnly && (user == nil || !user.IsAdmin()) {
            continue
        }
        if item.AuthOnly && user == nil {
            continue
        }
        items = append(items, item)
    }
    return items
}
```

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
- **`FieldChoices`** — dynamic select dropdowns for foreign key fields, loaded from the database at request time
- **`HasAdmin`** — interface for apps to contribute admin routes and navigation
- **`verbose` struct tags** — provide human-readable column labels for the admin UI
- **`form:"-"`** — excludes fields (like relations) from the admin form

## Next

In [Part 7](part7.md), you'll add HTMX for smooth navigation and infinite scroll pagination.
