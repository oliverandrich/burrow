# Admin

Admin panel coordinator that discovers and mounts admin views from other apps.

**Package:** `github.com/oliverandrich/burrow/contrib/admin`

**Depends on:** `auth`

## Setup

```go
srv := burrow.NewServer(
    session.New(),
    auth.New(),
    admin.New(),
    staticApp, // staticfiles.New(myStaticFS) — returns (*App, error)
    // ... other apps
)
```

`admin.New()` uses built-in defaults for the layout and dashboard renderer. Use options to override with custom implementations:

```go
admin.New(admin.WithLayout(myCustomLayout), admin.WithDashboardRenderer(myCustomDashboard))
```

The admin app discovers admin views from other apps via the `HasAdmin` interface. Any app that implements `HasAdmin` gets its routes mounted under `/admin` with auth protection.

## Default Layout

The built-in default layout renders a full admin HTML page with Bootstrap 5 styling, a sidebar navigation, and htmx for SPA-like navigation via `hx-get`/`hx-target`.

The layout reads admin nav items from context and renders them in the sidebar. Static assets are served via the `staticfiles` app using content-hashed URLs.

**Note:** The `bootstrap` app must be registered to serve CSS/JS assets. The admin default layout references static files under the `"bootstrap"` prefix.

## ModelAdmin

The `admin/modeladmin` sub-package provides a generic, Django-style CRUD admin for any Bun model:

```go
import "github.com/oliverandrich/burrow/contrib/admin/modeladmin"

ma := &modeladmin.ModelAdmin[Note]{
    Slug:              "notes",
    DisplayName:       "Note",
    DisplayPluralName: "Notes",
    DB:                cfg.DB,
    ListFields:        []string{"ID", "Title", "CreatedAt"},
    OrderBy:           "created_at DESC",
    PageSize:          25,
    CanCreate:         true,
    CanEdit:           true,
    CanDelete:         true,
}
```

### The `verbose` Struct Tag

ModelAdmin uses the `verbose` struct tag to determine column headers and form labels:

```go
type Note struct {
    ID        int64     `bun:",pk,autoincrement" verbose:"ID"`
    Title     string    `verbose:"Title"`
    Body      string    `verbose:"Body"`
    CreatedAt time.Time `verbose:"Created"`
}
```

If no `verbose` tag is set, the Go field name is used as-is.

### The `form` Struct Tag

The `form` struct tag controls how fields appear in create/edit forms:

```go
type Article struct {
    ID     int64  `bun:",pk,autoincrement" verbose:"ID"`
    Title  string `verbose:"Title" form:"required"`
    Body   string `verbose:"Body" form:"widget=textarea"`
    Status string `verbose:"Status" form:"choices=draft|published|archived"`
}
```

| Option | Example | Description |
|--------|---------|-------------|
| `widget=<type>` | `form:"widget=textarea"` | Override the inferred input type |
| `choices=<a\|b\|c>` | `form:"choices=draft\|published"` | Render as `<select>` with pipe-separated options |
| `required` | `form:"required"` | Mark the field as required |
| `name=<field>` | `form:"name=custom_name"` | Custom form field name for POST binding |
| `-` | `form:"-"` | Skip the field entirely — not shown in forms |

Multiple options can be combined: `form:"required,widget=textarea"`.

**Type inference:** When no `widget` is specified, the form field type is inferred from the Go type:

| Go Type | Input Type |
|---------|------------|
| `bool` | `checkbox` |
| `int`, `int64`, etc. | `number` |
| `time.Time` | `date` |
| `string` | `text` |

**Auto-increment primary keys** are handled automatically: skipped entirely in create forms (the database assigns the value), and rendered as hidden fields in edit forms (preserving the value without allowing modification).

### Features

- **List view** with configurable columns, ordering, and offset pagination
- **Search** across text fields
- **Filters** with select dropdowns
- **Row actions** (e.g., retry, cancel, delete) with optional confirmation dialogs
- **Create/edit forms** auto-generated from model struct tags (`verbose` for labels)
- **i18n** — all labels are translatable via `LabelKey` fields and the i18n app
- **HTMX** — list navigation uses `hx-get`/`hx-target` for partial updates

### Search

Enable search by listing the database column names to search across:

```go
ma := &modeladmin.ModelAdmin[Note]{
    // ...
    SearchFields: []string{"title", "body"},
}
```

By default, search uses `LIKE` with `%term%` patterns, applied with OR logic across fields. Special characters (`%`, `_`, `\`) are escaped automatically.

#### FTS5 Auto-Detection

If you create an FTS5 virtual table following the `{tablename}_fts` naming convention (e.g., `notes_fts` for a `notes` table), ModelAdmin automatically detects it at boot time and uses FTS5 `MATCH` queries instead of `LIKE`. This gives you word-based matching, FTS5 query syntax (AND, OR, NOT, prefix), and better performance on large datasets — with zero configuration.

If the FTS5 query has syntax errors (e.g., unmatched quotes from user input), ModelAdmin falls back to LIKE automatically.

See the [Full-Text Search guide](../guide/fts5.md) for instructions on creating FTS5 tables and triggers.

### Filters

Add filters to the list view using `FilterDef`:

```go
ma := &modeladmin.ModelAdmin[Article]{
    // ...
    Filters: []modeladmin.FilterDef{
        {
            Field:   "status",
            Label:   "Status",
            Type:    "select",
            Choices: []modeladmin.Choice{
                {Value: "draft", Label: "Draft"},
                {Value: "published", Label: "Published"},
                {Value: "archived", Label: "Archived"},
            },
        },
        {
            Field: "featured",
            Label: "Featured",
            Type:  "bool",
        },
    },
}
```

| Filter Type | Description |
|-------------|-------------|
| `"select"` | Dropdown with predefined choices |
| `"bool"` | True/false toggle |

Filter labels support i18n via the `LabelKey` field — when set, the label is translated at request time using the i18n app.

### Sorting

Enable clickable column sorting by listing the allowed database column names:

```go
ma := &modeladmin.ModelAdmin[Note]{
    // ...
    SortFields: []string{"title", "created_at"},
    OrderBy:    "created_at DESC", // default order
}
```

Sort fields are rendered as clickable column headers in the list view. The query parameter `?sort=title` sorts ascending, `?sort=-title` sorts descending. Only columns listed in `SortFields` are accepted — unknown fields are ignored.

### Row Actions

```go
RowActions: []modeladmin.RowAction{
    {
        Slug:    "retry",
        Label:   "admin-jobs-action-retry",
        Icon:    bsicons.ArrowCounterclockwise(),
        Class:   "btn-outline-success",
        Handler: retryHandler,
        ShowWhen: func(j Job) bool { return j.Status == StatusFailed },
    },
},
```

### Custom Renderer

ModelAdmin uses a `Renderer[T]` interface for all view rendering:

```go
type Renderer[T any] interface {
    List(w http.ResponseWriter, r *http.Request, items []T, page burrow.PageResult, cfg RenderConfig) error
    Detail(w http.ResponseWriter, r *http.Request, item *T, cfg RenderConfig) error
    Form(w http.ResponseWriter, r *http.Request, item *T, fields []FormField, errors *burrow.ValidationError, cfg RenderConfig) error
    ConfirmDelete(w http.ResponseWriter, r *http.Request, item *T, cfg RenderConfig) error
}
```

The default renderer uses Bootstrap 5 HTML templates with htmx:

```go
import "github.com/oliverandrich/burrow/contrib/admin/modeladmin/templates"

ma := &modeladmin.ModelAdmin[Note]{
    // ...
    Renderer: templates.DefaultRenderer[Note](),
}
```

Override the renderer when you need custom detail views, alternative CSS frameworks, or specialised list layouts. Implement all four methods and set `Renderer` on the `ModelAdmin`.

### Registering in Your App

Implement `HasAdmin` and delegate to the ModelAdmin:

```go
func (a *App) AdminRoutes(r chi.Router) {
    a.notesAdmin.Routes(r)
}

func (a *App) AdminNavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {Label: "Notes", LabelKey: "admin-nav-notes", URL: "/admin/notes", Icon: bsicons.JournalText(), Position: 20},
    }
}
```

## Routes

The admin app creates the `/admin` route group with `auth.RequireAuth()` and `auth.RequireAdmin()` middleware, then delegates to each `HasAdmin` app.

The dashboard is available at `GET /admin/`.

## CLI Commands

The CLI subcommands for user management (`promote`, `demote`, `create-invite`) are contributed by the **auth** app via `HasCLICommands`, not by the admin app itself. See [Auth docs](auth.md) for details.

To wire up CLI commands from all apps, add them to your `cli.Command`:

```go
cmd := &cli.Command{
    Name:     "myapp",
    Flags:    srv.Flags(nil),
    Action:   srv.Run,
    Commands: srv.Registry().AllCLICommands(),
}
```

## HasAdmin Interface

Apps contribute admin views by implementing `HasAdmin`:

```go
type HasAdmin interface {
    AdminRoutes(r chi.Router)
    AdminNavItems() []NavItem
}
```

The admin app collects all `HasAdmin` implementations and mounts their routes under `/admin` with `auth.RequireAuth()` and `auth.RequireAdmin()` middleware.

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `HasRoutes` | Creates `/admin` group and delegates to `HasAdmin` apps |
| `HasTemplates` | Contributes admin layout and page templates |
| `HasFuncMap` | Contributes admin icon template functions |
| `HasTranslations` | Contributes English and German translations for admin UI |
| `HasDependencies` | Requires `auth` |
