# Part 6: Admin Panel

In this part you'll add an admin panel with automatic CRUD for questions using `ModelAdmin`.

**Source code:** [`tutorial/step06/`](https://github.com/oliverandrich/burrow/src/branch/main/tutorial/step06)

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

## Set Up ModelAdmin

`ModelAdmin` provides generic CRUD views for any Bun model. Add it to the polls app:

```go
import (
    "github.com/oliverandrich/burrow/contrib/admin/modeladmin"
    matpl "github.com/oliverandrich/burrow/contrib/admin/modeladmin/templates"
)

type App struct {
    repo           *Repository
    handlers       *Handlers
    questionsAdmin *modeladmin.ModelAdmin[Question]
}
```

Add `verbose` struct tags to the model so ModelAdmin knows how to label columns:

```go
type Question struct {
    bun.BaseModel `bun:"table:questions,alias:q"`

    ID          int64     `bun:",pk,autoincrement" verbose:"ID"`
    Text        string    `bun:",notnull" verbose:"Question"`
    PublishedAt time.Time `bun:",notnull,default:current_timestamp" verbose:"Published"`

    Choices []Choice `bun:"rel:has-many,join:id=question_id"`
}
```

Initialise the ModelAdmin in `Register()`:

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
    return nil
}
```

## Implement HasAdmin

The `HasAdmin` interface has two methods: `AdminRoutes()` and `AdminNavItems()`.

```go
func (a *App) AdminNavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {Label: "Questions", URL: "/admin/questions", Position: 30},
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
}
```

`AdminRoutes` receives a router scoped to `/admin/`, so the full path becomes `/admin/questions`.

## Run It

```bash
go run .
```

Register a user, then promote them to admin via the database:

```bash
sqlite3 app.db "UPDATE users SET is_admin = 1 WHERE id = 1"
```

Visit `/admin/` to see the dashboard. Click "Questions" in the sidebar to list, create, edit, and delete questions — all without writing any template code.

## What You've Learnt

- **`admin.New()`** — coordinates the admin panel with built-in default layout and dashboard
- **`ModelAdmin`** — generic CRUD views for any Bun model, configured declaratively
- **`HasAdmin`** — interface for apps to contribute admin routes and navigation
- **`verbose` struct tags** — provide human-readable column labels for the admin UI

## Next

In [Part 7](part7.md), you'll add HTMX for smooth navigation and infinite scroll pagination.
