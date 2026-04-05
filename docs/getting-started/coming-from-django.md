# Coming from Django

Burrow shares Django's "batteries-included" philosophy but takes a Go-idiomatic approach. If you've built Django apps, you already understand the concepts — this page maps them to their Burrow equivalents.

## Quick Reference

| Django | Burrow |
|--------|--------|
| `INSTALLED_APPS` | `burrow.NewServer(app1, app2, ...)` |
| `django.contrib.*` | `contrib/` packages |
| `models.Model` | Den document with `document.Base` embed |
| `Manager` / `QuerySet` | Repository pattern + Den QuerySet API |
| `ForeignKey` / `ManyToManyField` | Document references via ID fields |
| `forms.Form` | Struct with `form` + `validate` tags, `burrow.Bind()` |
| `django.template` | `html/template` with `{{ define }}` blocks |
| `{% extends %}` / `{% block %}` | Layout templates with `.Content` wrapping |
| `templatetags` | `HasFuncMap` / `HasRequestFuncMap` |
| `manage.py` commands | `urfave/cli` commands via `HasCLICommands` |
| `migrations` | Automatic schema from document structs via `HasDocuments` |
| `STATIC_URL` / `collectstatic` | `go:embed` + `staticfiles` contrib |
| `settings.py` | CLI flags + ENV vars + TOML via `Configurable` |
| `middleware` | `func(http.Handler) http.Handler` via `HasMiddleware` |
| `django.contrib.admin` | `contrib/admin` with `ModelAdmin` |
| `django.contrib.auth` | `contrib/auth` (WebAuthn/passkeys) |
| `@permission_required` | Middleware checks (e.g. `auth.RequireAuth()`) |
| `django.contrib.sessions` | `contrib/session` (gorilla/sessions) |
| `signals` | Explicit function calls (no automatic dispatch) |
| `wsgi` / `gunicorn` | Single binary with built-in HTTP server |

## Apps & Registration

Django discovers apps through `INSTALLED_APPS` in `settings.py` and uses `AppConfig` classes. Burrow passes apps directly to `NewServer()`:

```go
srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    bootstrap.New(),
    &notesApp{},
)
```

Every app implements the `App` interface (`Name()`). Optional interfaces like `HasRoutes`, `HasMiddleware`, `Configurable`, or `HasTemplates` add capabilities. The framework auto-sorts apps by their declared dependencies — no manual ordering needed.

## Models & Database

Django uses `models.Model` with ORM magic — managers, querysets, `makemigrations`. Burrow uses [Den](https://github.com/oliverandrich/den) documents with struct tags and a functional query API:

=== "Django"

    ```python
    class Note(models.Model):
        title = models.CharField(max_length=200)
        content = models.TextField()
        created_at = models.DateTimeField(auto_now_add=True)

    # Query
    notes = Note.objects.filter(title__contains="go").order_by("-created_at")
    ```

=== "Burrow"

    ```go
    type Note struct {
        document.Base
        Title     string    `json:"title" den:"index"`
        Content   string    `json:"content"`
        CreatedAt time.Time `json:"created_at"`
    }

    // Query
    notes, err := den.NewQuery[Note](ctx, db,
        where.Field("title").StringContains("go"),
    ).Sort("created_at", den.Desc).All()
    ```

Django relationships (`ForeignKey`, `ManyToManyField`) create automatic reverse accessors and lazy loading. Den provides typed references via `Link[T]` and reverse queries via `BackLinks`:

```go
type Note struct {
    document.Base
    Title  string         `json:"title"`
    Author den.Link[User] `json:"author"`
}

// Create with a linked author
note := &Note{Title: "Hello", Author: den.NewLink(&user)}
den.Insert(ctx, db, note)

// Fetch with links resolved (like Django's select_related)
note, _ := den.NewQuery[Note](ctx, db).WithFetchLinks().First()
fmt.Println(note.Author.Value.Name) // loaded automatically

// Reverse query (like Django's note_set.all())
notes, _ := den.BackLinks[Note](ctx, db, "author", userID)
```

`Link[T]` stores only the ID in JSON — the linked document is fetched on demand via `WithFetchLinks()` or `FetchLink()`. `BackLinks` finds all documents that reference a given target, similar to Django's reverse accessors.

Den provides a chainable QuerySet API — `den.NewQuery`, `den.FindByID`, etc. Queries execute when a terminal method (`.All()`, `.First()`, `.Count()`, `.Exists()`) is called:

```go
count, err := den.NewQuery[Note](ctx, db, where.Field("author_id").Eq(userID)).Count()
```

Django's `get_object_or_404()` maps to a fetch + error check pattern:

```go
note, err := den.FindByID[Note](ctx, db, id)
if err != nil {
    if errors.Is(err, den.ErrNotFound) {
        return burrow.NewHTTPError(http.StatusNotFound, "note not found")
    }
    return err
}
```

Schema is managed automatically from document structs — no hand-written SQL migration files. See [Migrations](../guide/migrations.md).

## Forms & Validation

Django provides `forms.Form` and `ModelForm` with field definitions, `is_valid()`, and `cleaned_data`. Burrow uses struct tags with `burrow.Bind()`:

=== "Django"

    ```python
    class NoteForm(forms.Form):
        title = forms.CharField(max_length=200)
        content = forms.CharField(widget=forms.Textarea)

    def create_note(request):
        form = NoteForm(request.POST)
        if form.is_valid():
            # use form.cleaned_data
    ```

=== "Burrow"

    ```go
    type CreateNoteRequest struct {
        Title   string `form:"title" validate:"required,max=200"`
        Content string `form:"content" validate:"required"`
    }

    func createNote(w http.ResponseWriter, r *http.Request) error {
        var req CreateNoteRequest
        if err := burrow.Bind(r, &req); err != nil {
            return err // returns 422 with validation errors
        }
        // use req.Title, req.Content
    }
    ```

In Django, `form.is_valid()` returns `False` and you re-render the template with `form.errors`. In Burrow, catch the `*ValidationError` and re-render with the user's input preserved:

```go
if err := burrow.Bind(r, &req); err != nil {
    var ve *burrow.ValidationError
    if errors.As(err, &ve) {
        return burrow.Render(w, r, http.StatusUnprocessableEntity, "notes/form", map[string]any{
            "Form":   req,  // preserve user input
            "Errors": ve,   // per-field errors
        })
    }
    return err
}
```

There is no form rendering — you write the HTML yourself. See [Validation](../guide/validation.md) for the full pattern including template-side error display.

## Templates

This is the biggest mental model shift from Django. Four key differences:

### No Template Inheritance

Django uses `{% extends "base.html" %}` with `{% block content %}` to build pages from a base template. Burrow doesn't have template inheritance at all. Instead, a **layout template** wraps your rendered content in an HTML shell:

=== "Django"

    ```html
    {# base.html #}
    <html>
    <body>
      <nav>...</nav>
      {% block content %}{% endblock %}
    </body>
    </html>

    {# notes/list.html #}
    {% extends "base.html" %}
    {% block content %}
      <h1>Notes</h1>
      ...
    {% endblock %}
    ```

=== "Burrow"

    ```html
    {{/* templates/notes/list.html */}}
    {{ define "notes/list" -}}
    <h1>Notes</h1>
    ...
    {{- end }}
    ```

    The layout is a template name set once on the server — not declared in each template:

    ```go
    srv.SetLayout("myapp/layout")
    ```

Templates only define their own content. `Render` renders the page template, then renders the layout template with `.Content` set to the rendered fragment — unlike Django's blocks, you can't inject content into multiple slots. Dynamic data (navigation, current user, etc.) is accessed via template functions like `navLinks`, `currentUser`, `csrfToken`. If you need reusable parts within a page, use `{{ template "name" . }}` calls (similar to Django's `{% include %}`). See [Layouts & Rendering](../guide/layouts.md) for details.

### Named Blocks Instead of Includes

Django uses `{% include "partials/card.html" %}`. Burrow uses `{{ define }}` blocks and `{{ template }}` calls — all templates are named fragments in one global set:

```html
{{ define "notes/card" -}}
<div class="card">
  <h3>{{ .Title }}</h3>
  <p>{{ .Content }}</p>
</div>
{{- end }}

{{ define "notes/list" -}}
<h1>Notes</h1>
{{ range .Notes }}
  {{ template "notes/card" . }}
{{ end }}
{{- end }}
```

### No Template Discovery

Django walks `DIRS` and app `templates/` directories to find templates. Burrow collects templates from apps that implement `HasTemplates` at boot time — each app provides an `embed.FS`:

```go
//go:embed templates/*.html
var templateFS embed.FS

func (a *notesApp) Templates() fs.FS {
    return templateFS
}
```

### FuncMap Instead of Template Tags

Django uses `{% load %}` to import tag libraries. Burrow registers functions globally via `HasFuncMap` (static) or `HasRequestFuncMap` (per-request). No `{% load %}` needed — all functions are always available in every template:

=== "Django"

    ```html
    {% load my_tags %}
    {{ value|my_filter }}
    {% my_tag arg1 arg2 %}
    ```

=== "Burrow"

    ```html
    {{ myFunc .Value }}
    {{ staticURL "app/style.css" }}
    {{ csrfToken }}
    ```

See [Template Functions](../reference/template-functions.md) for the complete list.

## Configuration

Django uses a `settings.py` module with Python constants. Burrow layers three config sources with priority:

1. **CLI flags** (highest priority)
2. **Environment variables**
3. **TOML config file** (lowest priority)

Apps declare their own flags via the `HasFlags` interface. Values are read in the `Configure()` callback (`Configurable` interface):

```go
func (a *myApp) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
    return []cli.Flag{
        &cli.StringFlag{Name: "my-api-key", Sources: cli.EnvVars("MY_API_KEY")},
    }
}

func (a *myApp) Configure(cfg *burrow.AppConfig, cmd *cli.Command) error {
    a.apiKey = cmd.String("my-api-key")
    return nil
}
```

For multi-environment setups (like Django's `settings/dev.py` and `settings/prod.py`), use separate TOML files and environment variables for secrets — similar to Django's 12-factor pattern. See [Configuration](../guide/configuration.md) for the full guide.

## Middleware

Django uses class-based middleware with `process_request`, `process_response`, and `process_view` hooks. Burrow uses the stdlib wrapper pattern:

=== "Django"

    ```python
    class TimingMiddleware:
        def __init__(self, get_response):
            self.get_response = get_response

        def __call__(self, request):
            start = time.time()
            response = self.get_response(request)
            duration = time.time() - start
            response["X-Duration"] = str(duration)
            return response
    ```

=== "Burrow"

    ```go
    func TimingMiddleware(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            next.ServeHTTP(w, r)
            w.Header().Set("X-Duration", time.Since(start).String())
        })
    }
    ```

Apps contribute middleware via `HasMiddleware`. The signature is always `func(http.Handler) http.Handler`.

## Deployment

Django requires a WSGI/ASGI server (Gunicorn, uvicorn), a reverse proxy (Nginx), a process manager, and an external database (PostgreSQL). Burrow compiles to a single binary:

```bash
go build -o myapp .
./myapp --addr :8080
```

That binary includes:

- **HTTP server** with graceful shutdown
- **SQLite database** (no external database needed)
- **TLS support** (ACME via Let's Encrypt, manual certs, or off)
- **All static assets** embedded at compile time

No virtualenv, no pip, no process manager, no `collectstatic`.

## What's Different

Key philosophical differences from Django:

- **Explicit over implicit** — no ORM magic, no auto-discovery, no metaclasses. Queries are explicit function calls, config is flags, wiring is function calls.
- **Compile-time safety** — type errors are caught at build time, not at runtime when a user hits a page.
- **Single binary deployment** — no virtualenv, no pip, no process manager, no external database server.
- **SQLite or PostgreSQL** — two backends, same API. SQLite for single-binary deploys, PostgreSQL for scale. Switch with `--database-dsn`.
- **No admin auto-generation** — Django introspects your models and auto-generates CRUD forms, list views, and search. Burrow's `ModelAdmin` requires you to manually specify which fields are displayed, editable, and searchable — more work, but fully explicit. Django's `__str__` maps to Go's `fmt.Stringer` interface — implement `String()` on your document types and `ModelAdmin` uses it to display labels in list views.
- **Context instead of thread-locals** — `context.Context` replaces Django's `request.user` magic and thread-local storage. Values flow explicitly through the call chain.
- **No signals** — Django dispatches `post_save`, `pre_delete`, etc. automatically via the ORM. Burrow has no automatic lifecycle hooks — you call functions explicitly in your handlers or services. Use `Registry.Get()` for cross-app communication.
- **No built-in permission system** — Django has model-level permissions and `@permission_required`. Burrow provides authentication middleware (`auth.RequireAuth()`) but authorization logic is your responsibility — write middleware or handler checks.
- **No form rendering** — Django renders form fields as HTML. Burrow handles binding and validation; you write the HTML yourself.
- **No interactive shell** — Django's `manage.py shell` lets you experiment with models interactively. Go is compiled — exploratory work happens through tests and handlers, not a REPL.
