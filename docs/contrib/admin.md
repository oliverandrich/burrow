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

`admin.New()` uses built-in defaults for the layout template and dashboard renderer. Use options to override:

```go
admin.New(admin.WithLayout("myapp/admin-layout"), admin.WithDashboardRenderer(myCustomDashboard))
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

### Dynamic Select Dropdowns (FieldChoices)

For foreign key fields, use `FieldChoices` to render a `<select>` dropdown with options loaded from the database at request time:

```go
ma := &modeladmin.ModelAdmin[Choice]{
    // ...
    FieldChoices: map[string]modeladmin.ChoicesFunc{
        "QuestionID": func(ctx context.Context) ([]modeladmin.Choice, error) {
            var questions []Question
            err := db.NewSelect().Model(&questions).
                Order("title ASC").Scan(ctx)
            if err != nil {
                return nil, err
            }
            choices := make([]modeladmin.Choice, len(questions))
            for i, q := range questions {
                choices[i] = modeladmin.Choice{
                    Value: strconv.FormatInt(q.ID, 10),
                    Label: q.Title,
                }
            }
            return choices, nil
        },
    },
}
```

The key in the map is the Go struct field name (not the database column). The function is called on every form render, so new records appear automatically. This overrides the field's inferred type to `select`.

Unlike `form:"choices=a|b|c"` which defines static options, `FieldChoices` loads options dynamically — ideal for foreign keys and any field whose options come from the database.

### Read-Only Fields

Use `ReadOnlyFields` to display certain fields as non-editable plain text in create/edit forms. This is useful for fields set by the system (e.g. timestamps, computed values) that users should see but not modify:

```go
ma := &modeladmin.ModelAdmin[Article]{
    // ...
    ReadOnlyFields: []string{"CreatedAt", "UpdatedAt"},
}
```

Read-only fields are rendered as normal form controls with the `disabled` attribute, keeping the visual consistency of the form. Since disabled inputs don't submit values, the original model value is preserved — there is no risk of the value being blanked out by the POST.

Under the hood, this uses `forms.WithReadOnly`, which also strips validation errors for read-only fields so that `validate` tags on the struct don't cause false failures.

`ReadOnlyFields` overrides `form:"-"` — a field normally hidden from forms will appear as read-only in the admin. This lets you keep a field hidden in user-facing forms while showing it in the admin panel (e.g. an eager-loaded relation like `User`).

### Foreign Key Labels via `fmt.Stringer`

When a list field points to an eager-loaded relation (e.g. `User` instead of `UserID`), the list view checks if the value implements `fmt.Stringer`. If it does, `String()` is called and the result is displayed instead of the raw struct.

```go
// 1. Implement fmt.Stringer on the related model:
func (u User) String() string {
    if u.Name != "" {
        return u.Name
    }
    return u.Username
}

// 2. Add a belongs-to relation on the model:
type Note struct {
    bun.BaseModel `bun:"table:notes,alias:n"`
    UserID int64      `bun:",notnull"`
    User   *auth.User `bun:"rel:belongs-to,join:user_id=id" form:"-" verbose:"User"`
    // ...
}

// 3. Configure ModelAdmin to show the relation and eager-load it:
ma := &modeladmin.ModelAdmin[Note]{
    ListFields: []string{"ID", "Title", "User", "CreatedAt"},
    Relations:  []string{"User"},
    OrderBy:    "n.created_at DESC", // qualify with table alias when using relations
    // ...
}
```

The list view now shows the user's name instead of a numeric ID. The `User` field must be in `Relations` so Bun eager-loads it.

!!! note
    When using `Relations`, qualify ambiguous column names in `OrderBy` with the table alias (e.g. `n.created_at` instead of `created_at`), since joined tables may share column names like `id` or `created_at`.

### Computed List Columns (`ListDisplay`)

For columns that aren't direct struct fields — like derived values, counts, or formatted badges — use `ListDisplay`:

```go
ma := &modeladmin.ModelAdmin[Question]{
    ListFields: []string{"ID", "Text", "ChoiceCount", "PublishedAt"},
    Relations:  []string{"Choices"},
    ListDisplay: map[string]func(Question) template.HTML{
        "ChoiceCount": func(q Question) template.HTML {
            return template.HTML(fmt.Sprintf("<span>%d choices</span>", len(q.Choices)))
        },
    },
    // ...
}
```

Computed columns can be mixed freely with regular fields in `ListFields`. They take priority — if a computed column has the same name as a struct field, the computed function is called instead of reading the field value.

Computed columns are not sortable since they have no database column.

### Export

Enable CSV and JSON export of list views with `CanExport`:

```go
ma := &modeladmin.ModelAdmin[Note]{
    // ...
    CanExport: true,
}
```

When enabled, an "Export" dropdown appears next to the "New" button. Exports respect the current search query, filters, and sort order — but skip pagination to return all matching rows.

Files are downloaded as `{slug}-{date}.csv` or `{slug}-{date}.json`. Column headers use the Go struct field names from `ListFields`. Computed columns (`ListDisplay`) are skipped in exports since they produce HTML, not plain text.

### Delete Confirmation

When `CanDelete` is true, clicking the delete button navigates to a dedicated confirmation page (`GET /{slug}/{id}/delete`) instead of showing an inline browser confirm dialog. This gives users a clear chance to review what they're about to delete.

**Cascade impact detection** is automatic: at boot time, ModelAdmin introspects SQLite foreign keys to find tables with `ON DELETE CASCADE` referencing the model's table. When cascades exist, the confirmation page shows how many related rows will also be deleted (e.g., "5 × comments", "2 × attachments").

- No configuration needed — cascade detection works out of the box
- Only `ON DELETE CASCADE` foreign keys are detected; `SET NULL`, `RESTRICT`, etc. are ignored
- If no cascades exist, the confirmation page shows a simple "Are you sure?" message
- The actual deletion still uses `DELETE /{slug}/{id}` (unchanged)

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
