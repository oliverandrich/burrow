# Forms

The `forms` package provides generic, type-safe HTML form handling. It extracts field metadata from struct tags, binds request data, validates input, and produces renderable field objects for templates — eliminating the manual wiring between `Bind()`, validation errors, and template re-rendering.

Use `forms` when you have a model-backed HTML form. For JSON APIs or standalone validation without template rendering, use [`Bind()` and `Validate()` directly](validation.md).

## Defining a Form

The forms package works with any Go struct. There are two common patterns:

**Using the model directly** — works well when the form fields closely match the model. Exclude non-editable fields with `form:"-"` or `WithExclude`:

```go
// Note is a Bun model. Form tags control how it behaves in forms.
type Note struct {
    ID        int64     `bun:",pk,autoincrement" form:"-"`
    Title     string    `bun:",notnull" form:"title" validate:"required" verbose:"Title"`
    Content   string    `bun:",notnull" form:"content" widget:"textarea" verbose:"Content"`
    CreatedAt time.Time `bun:",nullzero" form:"-"`
}
```

**Using a dedicated form struct** — better when the form diverges from the model (different fields, extra validation, computed values):

```go
// NoteForm is a dedicated form struct, separate from the Bun model.
type NoteForm struct {
    Title    string `form:"title" validate:"required,max=200" verbose:"Title"`
    Content  string `form:"content" widget:"textarea" verbose:"Content"`
    Category string `form:"category" choices:"general|work|personal" verbose:"Category"`
}
```

With a dedicated form struct, you map between form and model in your handler after validation.

### Struct Tags

| Tag | Purpose | Example |
|-----|---------|---------|
| `form` | HTML field name for binding. Use `-` to exclude a field. | `form:"title"` |
| `validate` | Validation rules ([validator tags](validation.md#validation-tags)) | `validate:"required,min=3"` |
| `verbose_name` or `verbose` | Human-readable label for templates | `verbose:"Title"` |
| `widget` | Force HTML input type | `widget:"textarea"` |
| `help_text` | Help text shown below the field | `help_text:"Enter a short title"` |
| `choices` | Static select options (pipe-separated) | `choices:"draft\|published"` |

Fields are skipped when they are unexported, anonymous (embedded), or tagged with `form:"-"`.

### Widget Types

If no `widget` tag is set, the type is inferred from the Go type:

| Go type | Default widget |
|---------|---------------|
| `string` | `text` |
| `bool` | `checkbox` |
| `int`, `float64`, etc. | `number` |
| `time.Time` | `date` |

Fields with `choices` tag or dynamic choices automatically become `select`. You can override any inferred type with the `widget` tag. Supported values: `text`, `textarea`, `number`, `select`, `checkbox`, `date`, `email`, `hidden`.

## Creating Forms

### Using a model directly

When the form struct is your Bun model, use `New` for create pages and `FromModel` for edit pages. Non-editable fields like `ID` or `CreatedAt` are excluded via options:

```go
// Create page — empty form
f := forms.New[Note](forms.WithExclude[Note]("ID", "CreatedAt"))

// Edit page — pre-populated from existing record
note, _ := repo.Get(ctx, id)
f := forms.FromModel(note, forms.WithExclude[Note]("ID", "CreatedAt"))
```

!!! tip "Reuse options across handlers"
    If you use the same options in multiple handlers, extract them into a helper function to avoid repetition. See the [notes example app](https://codeberg.org/oliverandrich/burrow/src/branch/main/example/notes) for this pattern.

After validation, `f.Instance()` returns the model directly — ready to pass to your repository.

### Using a dedicated form struct

When the form struct is separate from the model, use `New` for both create and edit. For edit pages, populate the form with `WithInitial`:

```go
// Create page — empty form
f := forms.New[NoteForm]()

// Edit page — populate from existing record
note, _ := repo.Get(ctx, id)
f := forms.New[NoteForm](
    forms.WithInitial[NoteForm](map[string]any{
        "title":    note.Title,
        "content":  note.Content,
        "category": note.Category,
    }),
)
```

After validation, `f.Instance()` returns a `*NoteForm` — map it back to your model in the handler:

```go
form := f.Instance()
note.Title = form.Title
note.Content = form.Content
note.Category = form.Category
```

## Options

Options are passed to `New` or `FromModel` to configure form behavior.

### WithExclude

Hides fields from the form. Excluded fields won't appear in `Fields()` and won't be bound from the request. Use Go struct field names (not form tag names):

```go
forms.WithExclude[Note]("ID", "UserID", "CreatedAt")
```

This is the typical way to keep database-managed fields out of user-facing forms when using a model directly.

### WithInitial

Sets initial field values for the form. Use form tag names (not Go struct field names) as keys. Values appear in `BoundField.Value` before the form is submitted:

```go
forms.WithInitial[NoteForm](map[string]any{
    "title":    "Untitled",
    "category": "general",
})
```

Particularly useful with dedicated form structs to populate edit forms from an existing record (see [Creating Forms](#using-a-dedicated-form-struct)).

### WithReadOnly

Marks fields as read-only. Read-only fields appear in `Fields()` but are not editable — templates should render them with the `disabled` attribute. On `Bind()`, their values are preserved from the original instance (not overwritten by the request), and validation errors for these fields are stripped automatically.

```go
forms.WithReadOnly[Note]("CreatedAt", "UpdatedAt")
```

Read-only overrides `form:"-"` — a field tagged with `form:"-"` is normally hidden, but `WithReadOnly` forces it to appear. This is useful when the same model is used in different contexts (e.g. a user-facing form hides the field, but an admin form shows it as read-only).

### WithChoices

Provides static choices for a select field. Use the Go struct field name:

```go
forms.WithChoices[Article]("Status", []forms.Choice{
    {Value: "draft", Label: "Draft"},
    {Value: "published", Label: "Published"},
    {Value: "archived", Label: "Archived"},
})
```

This is an alternative to the `choices` struct tag when you need `Choice` structs with separate values and labels, or when choices are defined outside the struct.

### WithChoicesFunc

Provides a function that loads choices dynamically at bind time. The function receives the request context, so it can query a database or call a service:

```go
forms.WithChoicesFunc[Article]("CategoryID", func(ctx context.Context) ([]forms.Choice, error) {
    categories, err := repo.ListCategories(ctx)
    if err != nil {
        return nil, err
    }
    choices := make([]forms.Choice, len(categories))
    for i, c := range categories {
        choices[i] = forms.Choice{Value: strconv.Itoa(c.ID), Label: c.Name}
    }
    return choices, nil
})
```

See also the [ChoiceProvider interface](#dynamic-choices-via-choiceprovider-interface) for an alternative that keeps the logic on the struct itself.

## Binding and Validation

Call `Bind()` with the incoming request. It decodes form data, runs validation, and returns whether the form is valid:

```go
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) error {
    f := forms.New[Note](noteFormOpts()...)

    if !f.Bind(r) {
        // Re-render form with errors
        return burrow.Render(w, r, http.StatusUnprocessableEntity, "myapp/form", map[string]any{
            "Fields":         f.Fields(),
            "NonFieldErrors": f.NonFieldErrors(),
            "Action":         "/notes",
        })
    }

    note := f.Instance()
    // ... save note
}
```

`Bind()` performs these steps in order:

1. Decodes the request body via `burrow.Bind()` (form-encoded or JSON)
2. Runs per-field validation from `validate` tags
3. Loads dynamic choices (from `ChoiceProvider` or `WithChoicesFunc`)
4. Runs cross-field validation via `Clean()` (if implemented)

### Accessing Results

| Method | Returns |
|--------|---------|
| `f.Bind(r)` | `bool` — true if valid |
| `f.IsValid()` | `bool` — same result after `Bind()` |
| `f.Instance()` | `*T` — the bound struct with validated values |
| `f.Errors()` | `*burrow.ValidationError` — nil if valid |
| `f.NonFieldErrors()` | `[]string` — cross-field errors from `Clean()` |
| `f.Fields()` | `[]BoundField` — all visible fields with values and errors |
| `f.Field("Name")` | `(BoundField, bool)` — single field by Go struct field name |

## Rendering in Templates

`Fields()` returns a slice of `BoundField` objects. Each field carries everything needed to render an input:

| Property | Type | Description |
|----------|------|-------------|
| `Name` | `string` | Go struct field name |
| `FormName` | `string` | HTML name attribute |
| `Label` | `string` | Human-readable label |
| `HelpText` | `string` | Help text for the field |
| `Type` | `string` | HTML input type (`text`, `textarea`, `select`, etc.) |
| `Value` | `any` | Current field value |
| `Required` | `bool` | Whether the field is required |
| `ReadOnly` | `bool` | Whether the field is read-only (render as disabled) |
| `Choices` | `[]Choice` | Options for select fields |
| `Errors` | `[]string` | Validation errors for this field |

### Example Template

```html
{{ define "myapp/form" -}}
<form method="post" action="{{ .Action }}">
    <input type="hidden" name="gorilla.csrf.Token" value="{{ csrfToken }}">

    {{ if .NonFieldErrors -}}
    <div class="alert alert-danger">
        {{ range .NonFieldErrors }}<p class="mb-0">{{ . }}</p>{{ end }}
    </div>
    {{- end }}

    {{ range .Fields -}}
    <div class="mb-3">
        <label for="{{ .FormName }}" class="form-label">
            {{ .Label }}{{ if .Required }} *{{ end }}
        </label>

        {{ if eq .Type "textarea" -}}
        <textarea class="form-control{{ if .Errors }} is-invalid{{ end }}"
            id="{{ .FormName }}" name="{{ .FormName }}" rows="3">{{ .Value }}</textarea>

        {{- else if eq .Type "select" -}}
        <select class="form-select{{ if .Errors }} is-invalid{{ end }}"
            id="{{ .FormName }}" name="{{ .FormName }}">
            <option value="">—</option>
            {{ range .Choices -}}
            <option value="{{ .Value }}"{{ if eq .Value $.Value }} selected{{ end }}>{{ .Label }}</option>
            {{- end }}
        </select>

        {{- else if eq .Type "checkbox" -}}
        <div class="form-check">
            <input type="checkbox" class="form-check-input{{ if .Errors }} is-invalid{{ end }}"
                id="{{ .FormName }}" name="{{ .FormName }}" value="true"{{ if .Value }} checked{{ end }}>
        </div>

        {{- else -}}
        <input type="{{ .Type }}" class="form-control{{ if .Errors }} is-invalid{{ end }}"
            id="{{ .FormName }}" name="{{ .FormName }}" value="{{ .Value }}"
            {{ if .Required }}required{{ end }}>
        {{- end }}

        {{ if .HelpText -}}
        <div class="form-text">{{ .HelpText }}</div>
        {{- end }}

        {{ range .Errors -}}
        <div class="invalid-feedback">{{ . }}</div>
        {{- end }}
    </div>
    {{- end }}

    <button type="submit" class="btn btn-primary">Save</button>
</form>
{{- end }}
```

## Select Fields and Choices

There are four ways to provide choices for a select field.

**Static choices via struct tag** — works on both models and form structs:

```go
type ArticleForm struct {
    Status string `form:"status" choices:"draft|published|archived" verbose:"Status"`
}
```

**Static choices via option** — when you need separate values and labels, or choices defined outside the struct:

```go
statuses := []forms.Choice{
    {Value: "draft", Label: "Draft"},
    {Value: "published", Label: "Published"},
    {Value: "archived", Label: "Archived"},
}
f := forms.New[ArticleForm](forms.WithChoices[ArticleForm]("Status", statuses))
```

**Dynamic choices via option function** — when choices come from a database or service:

```go
f := forms.New[ArticleForm](
    forms.WithChoicesFunc[ArticleForm]("CategoryID", func(ctx context.Context) ([]forms.Choice, error) {
        categories, err := repo.ListCategories(ctx)
        if err != nil {
            return nil, err
        }
        choices := make([]forms.Choice, len(categories))
        for i, c := range categories {
            choices[i] = forms.Choice{Value: strconv.Itoa(c.ID), Label: c.Name}
        }
        return choices, nil
    }),
)
```

**Dynamic choices via `ChoiceProvider` interface** — when you want the choice logic on the struct itself. The struct that implements the interface is whatever you pass as the type parameter to `New` or `FromModel` — a model or a dedicated form struct:

```go
// ArticleForm is a dedicated form struct with dynamic choices.
type ArticleForm struct {
    Title      string `form:"title" validate:"required" verbose:"Title"`
    CategoryID int    `form:"category_id" widget:"select" verbose:"Category"`
}

func (f *ArticleForm) FieldChoices(ctx context.Context, field string) ([]forms.Choice, error) {
    if field == "CategoryID" {
        // load categories from database
    }
    return nil, nil
}
```

When using a model directly, the same interface works — just implement it on the model:

```go
// Article is a Bun model that also provides its own choices.
type Article struct {
    ID         int64  `bun:",pk,autoincrement" form:"-"`
    Title      string `bun:",notnull" form:"title" validate:"required" verbose:"Title"`
    CategoryID int    `bun:",notnull" form:"category_id" widget:"select" verbose:"Category"`
}

func (a *Article) FieldChoices(ctx context.Context, field string) ([]forms.Choice, error) {
    if field == "CategoryID" {
        // load categories from database
    }
    return nil, nil
}
```

## Cross-Field Validation

There are two mechanisms for cross-field validation: `Clean(ctx)` on the struct itself, and `WithCleanFunc` as a form option. Use whichever fits your needs — or both.

### Clean(ctx) — struct-level validation

Implement the `Cleanable` interface for validation that only needs the struct's own data. The context carries request-scoped data (e.g. i18n localizer). Like `ChoiceProvider`, define `Clean()` on whatever struct you use as the form type parameter:

```go
type EventForm struct {
    Start time.Time `form:"start" validate:"required" verbose:"Start date"`
    End   time.Time `form:"end"   validate:"required" verbose:"End date"`
}

func (f *EventForm) Clean(ctx context.Context) error {
    if !f.End.After(f.Start) {
        return &burrow.ValidationError{
            Errors: []burrow.FieldError{
                {Field: "end", Message: "End date must be after start date"},
            },
        }
    }
    return nil
}
```

### WithCleanFunc — closure-based validation

Use `WithCleanFunc` when validation needs external dependencies like a database or repository. The closure captures dependencies at form creation time:

```go
f := forms.New[UserForm](
    forms.WithCleanFunc(func(ctx context.Context, u *UserForm) error {
        if u.Role != "admin" {
            lastAdmin, _ := repo.IsLastAdmin(ctx, u.ID)
            if lastAdmin {
                return &burrow.ValidationError{
                    Errors: []burrow.FieldError{
                        {Field: "role", Message: "Cannot demote the last admin"},
                    },
                }
            }
        }
        return nil
    }),
)
```

### How they work together

Both mechanisms run only after all per-field validations pass. `Clean(ctx)` runs first, then `WithCleanFunc`. Both can return errors, and all errors are merged:

- Field-level errors (with a `Field` value) appear on the corresponding `BoundField.Errors`
- Non-field errors (empty `Field`) are accessible via `f.NonFieldErrors()`

| Mechanism | Use when | Dependencies |
|-----------|----------|-------------|
| `Clean(ctx)` | Logic needs only the struct's own fields | None (self-contained) |
| `WithCleanFunc` | Logic needs external state (DB, repo, services) | Captured via closure |

## Complete Example

The [notes example app](https://codeberg.org/oliverandrich/burrow/src/branch/main/example/notes) demonstrates a full CRUD workflow with the forms package, including create/edit handlers, error re-rendering, and HTMX integration.
