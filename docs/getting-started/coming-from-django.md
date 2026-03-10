# Coming from Django

Burrow shares Django's "batteries-included" philosophy but takes a Go-idiomatic approach. If you've built Django apps, you already understand the concepts — this page maps them to their Burrow equivalents.

## Quick Reference

| Django | Burrow |
|--------|--------|
| `INSTALLED_APPS` | `burrow.NewServer(app1, app2, ...)` |
| `django.contrib.*` | `contrib/` packages |
| `models.Model` | Bun model with `bun.BaseModel` embed |
| `Manager` / `QuerySet` | Repository pattern + Bun query builder |
| `forms.Form` | Struct with `form` + `validate` tags, `burrow.Bind()` |
| `django.template` | `html/template` with `{{ define }}` blocks |
| `{% extends %}` / `{% block %}` | `LayoutFunc` wrapping |
| `templatetags` | `HasFuncMap` / `HasRequestFuncMap` |
| `manage.py` commands | `urfave/cli` commands via `HasCLICommands` |
| `migrations` | Embedded `.up.sql` files via `Migratable` |
| `STATIC_URL` / `collectstatic` | `go:embed` + `staticfiles` contrib |
| `settings.py` | CLI flags + ENV vars + TOML via `Configurable` |
| `middleware` | `func(http.Handler) http.Handler` via `HasMiddleware` |
| `django.contrib.admin` | `contrib/admin` with `ModelAdmin` |
| `django.contrib.auth` | `contrib/auth` (WebAuthn/passkeys) |
| `django.contrib.sessions` | `contrib/session` (gorilla/sessions) |
| `signals` | Direct function calls or `Registry.Get()` |
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

Every app implements the `App` interface (`Name()` + `Register()`). Optional interfaces like `HasRoutes`, `HasMiddleware`, or `HasTemplates` add capabilities. The framework auto-sorts apps by their declared dependencies — no manual ordering needed.

## Models & Database

Django uses `models.Model` with ORM magic — managers, querysets, `makemigrations`. Burrow uses [Bun](https://bun.uptrace.dev/) models with struct tags and explicit SQL:

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
        bun.BaseModel `bun:"table:notes"`
        ID        int64     `bun:",pk,autoincrement"`
        Title     string    `bun:",notnull"`
        Content   string    `bun:",notnull"`
        CreatedAt time.Time `bun:",nullzero,default:current_timestamp"`
    }

    // Query
    var notes []Note
    err := db.NewSelect().Model(&notes).
        Where("title LIKE ?", "%go%").
        OrderExpr("created_at DESC").
        Scan(ctx)
    ```

Migrations are hand-written SQL files embedded in each app, not auto-generated. See [Migrations](../guide/migrations.md).

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

There is no form rendering — you write the HTML yourself. See [Validation](../guide/validation.md).

## Templates

This is the biggest mental model shift from Django. Four key differences:

### No Template Inheritance

Django uses `{% extends "base.html" %}` with `{% block content %}` to build pages from a base template. Burrow doesn't have template inheritance at all. Instead, a `LayoutFunc` — a Go function — wraps your rendered content in an HTML shell:

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

    The layout is a Go function set once on the server — not declared in each template:

    ```go
    srv.SetLayout(myLayout())
    ```

Templates only define their own content. The layout function wraps it. See [Layouts & Rendering](../guide/layouts.md) for details.

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

Apps declare their own flags via the `Configurable` interface. Values are read in the `Configure()` callback:

```go
func (a *myApp) Flags() []cli.Flag {
    return []cli.Flag{
        &cli.StringFlag{Name: "my-api-key", Sources: cli.EnvVars("MY_API_KEY")},
    }
}

func (a *myApp) Configure(cmd *cli.Command) error {
    a.apiKey = cmd.String("my-api-key")
    return nil
}
```

See [Configuration](../guide/configuration.md) for the full guide.

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

- **Explicit over implicit** — no ORM magic, no auto-discovery, no metaclasses. Queries are SQL, config is flags, wiring is function calls.
- **Compile-time safety** — type errors are caught at build time, not at runtime when a user hits a page.
- **Single binary deployment** — no virtualenv, no pip, no process manager, no external database server.
- **SQLite by default** — no PostgreSQL/MySQL abstraction layer. One database engine, optimized for it.
- **No admin auto-generation** — Django builds admin from model metadata. Burrow's `ModelAdmin` requires explicit configuration of fields, columns, and queries.
- **Context instead of thread-locals** — `context.Context` replaces Django's `request.user` magic and thread-local storage. Values flow explicitly through the call chain.
- **No form rendering** — Django renders form fields as HTML. Burrow handles binding and validation; you write the HTML yourself.
