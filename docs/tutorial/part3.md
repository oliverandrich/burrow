# Part 3: Templates & Layouts

In this part you'll add HTML templates, a Bootstrap-styled layout, and views that render question lists and detail pages.

**Source code:** [`tutorial/step03/`](https://github.com/oliverandrich/burrow/tree/main/tutorial/step03)

## How Templates Work in Burrow

Burrow builds a **global template set** at startup by collecting templates from all apps that implement `HasTemplates`. Each template file uses `{{ define "appname/template" }}` to declare its name. When you call `RenderTemplate()`, it looks up the template by name, executes it, and wraps the result in a layout (if one is set).

## Add Templates to the Polls App

Create the template directory for the polls app:

```bash
mkdir -p internal/polls/templates/polls
```

### Implement the Interfaces

Add the following imports to `internal/polls/polls.go` (alongside the existing ones from Part 2):

```go
"fmt"
"net/http"
"strconv"

"github.com/go-chi/chi/v5"
```

Then add the interface implementations. The polls app now implements `HasTemplates`, `HasRoutes`, and `HasNavItems`:

```go
//go:embed templates
var templateFS embed.FS

func (a *App) TemplateFS() fs.FS {
    sub, _ := fs.Sub(templateFS, "templates")
    return sub
}

func (a *App) NavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {Label: "Polls", URL: "/polls", Position: 10},
    }
}
```

`TemplateFS()` returns the embedded `templates/` directory. Burrow walks this filesystem and parses all `.html` files into the global template set.

### Write the Templates

Create `internal/polls/templates/polls/list.html`:

```html
{{ define "polls/list" -}}
<div class="container py-4">
    <h1>Polls</h1>
    {{ if .Questions -}}
    <div class="list-group">
        {{ range .Questions -}}
        <a href="/polls/{{ .ID }}" class="list-group-item list-group-item-action">
            <div class="d-flex w-100 justify-content-between">
                <h5 class="mb-1">{{ .Text }}</h5>
                <small class="text-body-secondary">
                    {{ .PublishedAt.Format "2 Jan 2006" }}
                </small>
            </div>
        </a>
        {{ end -}}
    </div>
    {{ else -}}
    <div class="alert alert-info">No polls available yet.</div>
    {{ end -}}
</div>
{{- end }}
```

Create `internal/polls/templates/polls/detail.html`:

```html
{{ define "polls/detail" -}}
<div class="container py-4">
    <h1>{{ .Question.Text }}</h1>
    <ul class="list-group mb-3">
        {{ range .Question.Choices -}}
        <li class="list-group-item">{{ .Text }}</li>
        {{ end -}}
    </ul>
    <a href="/polls" class="btn btn-secondary">&laquo; Back to polls</a>
</div>
{{- end }}
```

And `internal/polls/templates/polls/results.html`:

```html
{{ define "polls/results" -}}
<div class="container py-4">
    <h1>Results: {{ .Question.Text }}</h1>
    <ul class="list-group mb-3">
        {{ range .Question.Choices -}}
        <li class="list-group-item d-flex justify-content-between align-items-center">
            {{ .Text }}
            <span class="badge text-bg-primary rounded-pill">
                {{ .Votes }} vote{{ if ne .Votes 1 }}s{{ end }}
            </span>
        </li>
        {{ end -}}
    </ul>
    <a href="/polls/{{ .Question.ID }}" class="btn btn-primary">Vote again</a>
    <a href="/polls" class="btn btn-secondary">&laquo; Back to polls</a>
</div>
{{- end }}
```

### Update the App Struct

The app needs a `handlers` field and must initialise it during `Register()`. Update the `App` struct and `Register()` method in `internal/polls/polls.go`:

```go
type App struct {
    repo     *Repository
    handlers *Handlers
}

func (a *App) Register(cfg *burrow.AppConfig) error {
    a.repo = NewRepository(cfg.DB)
    a.handlers = &Handlers{repo: a.repo}
    return nil
}
```

### Add Handlers and Routes

Still in `internal/polls/polls.go`, add the `Handlers` struct and route registration:

```go
type Handlers struct {
    repo *Repository
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
    questions, err := h.repo.ListQuestions(r.Context())
    if err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list questions")
    }
    return burrow.RenderTemplate(w, r, http.StatusOK, "polls/list", map[string]any{
        "Title":     "Polls",
        "Questions": questions,
    })
}

func (h *Handlers) Detail(w http.ResponseWriter, r *http.Request) error {
    id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
    if err != nil {
        return burrow.NewHTTPError(http.StatusBadRequest, "invalid question ID")
    }
    question, err := h.repo.GetQuestion(r.Context(), id)
    if err != nil {
        return burrow.NewHTTPError(http.StatusNotFound, "question not found")
    }
    return burrow.RenderTemplate(w, r, http.StatusOK, "polls/detail", map[string]any{
        "Title":    question.Text,
        "Question": question,
    })
}

func (h *Handlers) Results(w http.ResponseWriter, r *http.Request) error {
    id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
    if err != nil {
        return burrow.NewHTTPError(http.StatusBadRequest, "invalid question ID")
    }
    question, err := h.repo.GetQuestion(r.Context(), id)
    if err != nil {
        return burrow.NewHTTPError(http.StatusNotFound, "question not found")
    }
    return burrow.RenderTemplate(w, r, http.StatusOK, "polls/results", map[string]any{
        "Title":    fmt.Sprintf("Results: %s", question.Text),
        "Question": question,
    })
}

func (a *App) Routes(r chi.Router) {
    r.Route("/polls", func(r chi.Router) {
        r.Get("/", burrow.Handle(a.handlers.List))
        r.Get("/{id}", burrow.Handle(a.handlers.Detail))
        r.Get("/{id}/results", burrow.Handle(a.handlers.Results))
    })
}
```

## Create a Pages App with Layout

The **pages app** provides the site layout and homepage. Create the directories first:

```bash
mkdir -p internal/pages/templates/app
mkdir -p internal/pages/templates/pages
```

Create `internal/pages/pages.go`:

```go
package pages

import (
    "embed"
    "io/fs"
    "net/http"

    "github.com/oliverandrich/burrow"
    "github.com/go-chi/chi/v5"
)

//go:embed templates
var templateFS embed.FS

type App struct{}

func New() *App { return &App{} }

func (a *App) Name() string                       { return "pages" }
func (a *App) Register(_ *burrow.AppConfig) error  { return nil }

func (a *App) TemplateFS() fs.FS {
    sub, _ := fs.Sub(templateFS, "templates")
    return sub
}

func (a *App) NavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {Label: "Home", URL: "/", Position: 0},
    }
}

func (a *App) Routes(r chi.Router) {
    r.Get("/", burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
        return burrow.RenderTemplate(w, r, http.StatusOK, "pages/home", map[string]any{
            "Title": "Welcome to Polls",
        })
    }))
}
```

### The Layout Name

Still in `internal/pages/pages.go`, add the `Layout()` function. It returns the template name for the layout:

```go
func Layout() string {
    return "app/layout"
}
```

When `RenderTemplate()` is called:

1. It executes the named template (e.g. `"polls/list"`) to produce an HTML fragment
2. It checks if the request is an HTMX request — if so, it returns the fragment directly
3. Otherwise, it renders the layout template, passing the fragment as `.Content`

### The Layout Template

Create `internal/pages/templates/app/layout.html`:

```html
{{ define "app/layout" -}}
<!DOCTYPE html>
<html lang="en" data-bs-theme="light">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{ if .Title }}{{ .Title }} — {{ end }}Polls</title>
    {{ template "bootstrap/css" . }}
    {{ template "bootstrap/js" . }}
</head>
<body>
    <nav class="navbar navbar-expand-lg bg-body-tertiary mb-4">
        <div class="container">
            <a class="navbar-brand" href="/">Polls</a>
            <div class="collapse navbar-collapse">
                <ul class="navbar-nav">
                    {{ range navLinks -}}
                    <li class="nav-item">
                        <a class="nav-link{{ if .IsActive }} active{{ end }}" href="{{ .URL }}">{{ .Label }}</a>
                    </li>
                    {{ end -}}
                </ul>
            </div>
        </div>
    </nav>
    <main class="container">
        {{ .Content }}
    </main>
</body>
</html>
{{- end }}
```

The `{{ template "bootstrap/css" }}` and `{{ template "bootstrap/js" }}` calls include the Bootstrap stylesheet and JavaScript bundle. These are reusable templates provided by the `bootstrap` contrib app — internally they use `staticURL` to generate content-hashed URLs for cache busting.

### The Homepage Template

Create `internal/pages/templates/pages/home.html`:

```html
{{ define "pages/home" -}}
<div class="px-4 py-5 text-center">
    <h1 class="display-5 fw-bold">Welcome to Polls</h1>
    <div class="col-lg-6 mx-auto">
        <p class="lead mb-4">
            A simple polling application built with the burrow framework.
        </p>
        <a href="/polls" class="btn btn-primary btn-lg">View Polls &raquo;</a>
    </div>
</div>
{{- end }}
```

## Update main.go

Replace your `main.go` with:

```go
package main

import (
    "context"
    "embed"
    "log"
    "os"

    "github.com/oliverandrich/burrow"
    "github.com/oliverandrich/burrow/contrib/bootstrap"
    "github.com/oliverandrich/burrow/contrib/htmx"
    "github.com/oliverandrich/burrow/contrib/staticfiles"
    "github.com/urfave/cli/v3"

    "polls/internal/pages"
    "polls/internal/polls"
)

var emptyFS embed.FS

func main() {
    staticApp, err := staticfiles.New(emptyFS)
    if err != nil {
        log.Fatal(err)
    }

    srv := burrow.NewServer(
        staticApp,
        htmx.New(),
        bootstrap.New(),
        pages.New(),
        polls.New(),
    )

    srv.SetLayout(pages.Layout())

    cmd := &cli.Command{
        Name:    "polls",
        Usage:   "Polls tutorial application",
        Version: "0.3.0",
        Flags:   srv.Flags(nil),
        Action:  srv.Run,
    }

    if err := cmd.Run(context.Background(), os.Args); err != nil {
        log.Fatal(err)
    }
}
```

This replaces the `homepageApp` from Part 1 with proper apps:

- **`staticfiles`** — serves static files with content-hashed URLs
- **`htmx`** — provides the htmx JavaScript library (required by Bootstrap app)
- **`bootstrap`** — provides Bootstrap 5 CSS/JS as static assets
- **`pages`** — homepage and layout
- **`polls`** — now with templates and routes

## Run It

```bash
go mod tidy
go run .
```

Open `http://localhost:8080` — you'll see the Bootstrap-styled homepage. Click "View Polls" to see the (empty) polls list. There are no questions yet because we haven't added a way to create them.

!!! tip "Seeding test data"
    You can use the SQLite CLI to insert test data:
    ```bash
    sqlite3 app.db "INSERT INTO questions (text) VALUES ('What is your favourite colour?')"
    sqlite3 app.db "INSERT INTO choices (question_id, text) VALUES (1, 'Red'), (1, 'Blue'), (1, 'Green')"
    ```
    Refresh the page to see them appear.

## What You've Learnt

- **`HasTemplates`** — apps contribute `.html` template files to the global template set
- **`RenderTemplate()`** — renders a named template, automatically wrapping in a layout for normal requests and returning fragments for HTMX requests
- **Layout templates** — wrap page content in a full HTML document with navigation (via `navLinks` template function), scripts, and styles
- **`staticfiles`** and **`bootstrap`** — contrib apps handle CSS/JS assets with cache busting

## Next

In [Part 4](part4.md), you'll add a voting form with CSRF protection, flash messages, and the redirect-after-POST pattern.
