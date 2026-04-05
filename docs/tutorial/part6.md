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

In `internal/polls/polls.go`, add `verbose` struct tags to both models so ModelAdmin knows how to label columns:

```go
type Question struct {
    document.Base                              `verbose:"ID"`
    Text        string    `json:"text" den:"index" verbose:"Question"`
    PublishedAt time.Time `json:"published_at" verbose:"Published"`
}

// String returns the question text for display in admin list views (e.g. as FK label).
func (q Question) String() string { return q.Text }

type Choice struct {
    document.Base                              `verbose:"ID"`
    QuestionID string `json:"question_id" den:"index" verbose:"Question"`
    Text       string `json:"text" verbose:"Choice"`
    Votes      int    `json:"votes" verbose:"Votes"`
}
```

The `String()` method on `Question` tells ModelAdmin how to display a question when it appears as a foreign key reference in list views. Any type that implements `fmt.Stringer` is rendered using its `String()` result instead of the raw struct.

## Set Up ModelAdmin

`ModelAdmin` provides generic CRUD views for any Den document type. In `internal/polls/polls.go`, add the imports and update the `App` struct:

```go
import (
    "github.com/oliverandrich/den"
    "github.com/oliverandrich/burrow/contrib/admin/modeladmin"
    matpl "github.com/oliverandrich/burrow/contrib/admin/modeladmin/templates"
)

type App struct {
    repo           *Repository
    questionsAdmin *modeladmin.ModelAdmin[Question]
    choicesAdmin   *modeladmin.ModelAdmin[Choice]
}
```

Update the `Configure()` method in `internal/polls/polls.go` to initialise both ModelAdmins:

```go
func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
    a.repo = NewRepository(cfg.DB)

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
                questions, err := den.NewQuery[Question](ctx, cfg.DB).
                    Sort("published_at", den.Desc).
                    All()
                if err != nil {
                    return nil, err
                }
                choices := make([]modeladmin.Choice, len(questions))
                for i, q := range questions {
                    choices[i] = modeladmin.Choice{
                        Value: q.ID,
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

Register a user, then promote them to admin using the auth CLI command:

```bash
./polls promote --username your-username
```

Visit `/admin/` to see the dashboard. Click "Questions" in the sidebar to create a question, then click "Choices" to add choices for it — the question dropdown shows all available questions.

## What You've Learnt

- **`admin.New()`** — coordinates the admin panel with built-in default layout and dashboard
- **`ModelAdmin`** — generic CRUD views for any Den document type, configured declaratively
- **`fmt.Stringer` for FK labels** — implement `String()` on related document types to display human-readable labels instead of raw IDs in list views
- **`FieldChoices`** — dynamic select dropdowns for foreign key fields in forms, loaded from the database at request time
- **`FieldChoices`** — dynamic select dropdowns for foreign key fields in forms, loaded from the database at request time
- **`HasAdmin`** — interface for apps to contribute admin routes and navigation
- **`verbose` struct tags** — provide human-readable column labels for the admin UI
- **`form:"-"`** — excludes fields (like relations) from the admin form

## Next

In [Part 7](part7.md), you'll add HTMX for smooth navigation and infinite scroll pagination.
