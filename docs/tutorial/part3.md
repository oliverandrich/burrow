# Part 3: Templates & Layouts

In this part you'll add HTML templates, a layout backed by a small hand-written stylesheet, and views that render question lists and detail pages.

**Source code:** [`tutorial/step03/`](https://github.com/oliverandrich/burrow/tree/main/tutorial/step03)

## How Templates Work in Burrow

Burrow builds a **global template set** at startup by collecting templates from all apps that implement `HasTemplates`. Each template file uses `{{ define "appname/template" }}` to declare its name. When you call `Render()`, it looks up the template by name, executes it, and wraps the result in a layout (if one is set).

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

!!! tip "Template errors"
    If a template has a syntax error, the server will fail to start and print the parsing error to the console. Fix the template and restart — there is no need to clear a cache.

### Write the Templates

Create `internal/polls/templates/polls/list.html`:

```html
{{ define "polls/list" -}}
<header>
    <h1>Polls</h1>
</header>
{{ if .Questions -}}
<div class="polls-list">
    {{ range .Questions -}}
    <a href="/polls/{{ .ID }}" class="polls-list-item">
        <article>
            <strong>{{ .Text }}</strong>
            <small>{{ .PublishedAt.Format "2 Jan 2006" }}</small>
        </article>
    </a>
    {{ end -}}
</div>
{{ else -}}
<div class="alert alert-info" role="alert">No polls available yet.</div>
{{ end -}}
<style>
.polls-list{display:flex;flex-direction:column;gap:.5rem}
.polls-list-item{color:inherit;text-decoration:none}
.polls-list-item article{display:flex;justify-content:space-between;align-items:baseline;gap:1rem;margin:0}
</style>
{{- end }}
```

Create `internal/polls/templates/polls/detail.html`:

```html
{{ define "polls/detail" -}}
<header>
    <h1>{{ .Question.Text }}</h1>
</header>
<ul>
    {{ range .Question.Choices -}}
    <li>{{ .Text }}</li>
    {{ end -}}
</ul>
<a href="/polls" role="button" class="btn btn-outline btn-secondary">&laquo; Back to polls</a>
{{- end }}
```

And `internal/polls/templates/polls/results.html`:

```html
{{ define "polls/results" -}}
<header>
    <h1>Results: {{ .Question.Text }}</h1>
</header>
<ul class="poll-results">
    {{ range .Question.Choices -}}
    <li>
        <span>{{ .Text }}</span>
        <span class="badge badge-primary">{{ .Votes }} vote{{ if ne .Votes 1 }}s{{ end }}</span>
    </li>
    {{ end -}}
</ul>
<div role="group">
    <a href="/polls/{{ .Question.ID }}" role="button" class="btn btn-primary">Vote again</a>
    <a href="/polls" role="button" class="btn btn-outline btn-secondary">&laquo; Back to polls</a>
</div>
<style>
.poll-results{list-style:none;padding:0}
.poll-results li{display:flex;justify-content:space-between;align-items:center;padding:.5rem 0;border-bottom:1px solid var(--mu-muted-border-color)}
</style>
{{- end }}
```

### Add Handlers and Routes

Still in `internal/polls/polls.go`, add handler methods on `*App` and route registration. Handlers are methods on the app itself, so they have direct access to the repository:

```go
func (a *App) List(w http.ResponseWriter, r *http.Request) error {
    questions, err := a.repo.ListQuestions(r.Context())
    if err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list questions")
    }
    return burrow.Render(w, r, http.StatusOK, "polls/list", map[string]any{
        "Title":     "Polls",
        "Questions": questions,
    })
}

func (a *App) Detail(w http.ResponseWriter, r *http.Request) error {
    id := chi.URLParam(r, "id")
    question, err := a.repo.GetQuestion(r.Context(), id)
    if err != nil {
        return burrow.NewHTTPError(http.StatusNotFound, "question not found")
    }
    return burrow.Render(w, r, http.StatusOK, "polls/detail", map[string]any{
        "Title":    question.Text,
        "Question": question,
    })
}

func (a *App) Results(w http.ResponseWriter, r *http.Request) error {
    id := chi.URLParam(r, "id")
    question, err := a.repo.GetQuestion(r.Context(), id)
    if err != nil {
        return burrow.NewHTTPError(http.StatusNotFound, "question not found")
    }
    return burrow.Render(w, r, http.StatusOK, "polls/results", map[string]any{
        "Title":    fmt.Sprintf("Results: %s", question.Text),
        "Question": question,
    })
}

func (a *App) Routes(r chi.Router) {
    r.Route("/polls", func(r chi.Router) {
        r.Get("/", burrow.Handle(a.List))
        r.Get("/{id}", burrow.Handle(a.Detail))
        r.Get("/{id}/results", burrow.Handle(a.Results))
    })
}
```

## Create an App Shell

The **app shell** provides the site layout, homepage, and the project-level stylesheet. Convention: it lives under `internal/app/`. Create the directories first:

```bash
mkdir -p internal/app/templates/app
mkdir -p internal/app/templates/pages
mkdir -p internal/app/static
```

Create `internal/app/app.go`:

```go
package app

import (
    "embed"
    "io/fs"
    "net/http"

    "github.com/oliverandrich/burrow"
    "github.com/go-chi/chi/v5"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

type App struct{}

func New() *App { return &App{} }

func (a *App) Name() string { return "app" }

func (a *App) TemplateFS() fs.FS {
    sub, _ := fs.Sub(templateFS, "templates")
    return sub
}

// StaticFS publishes the project stylesheet under the "app" prefix so the
// layout can link to it as {{ staticURL "app/app.css" }}.
func (a *App) StaticFS() (string, fs.FS) {
    sub, _ := fs.Sub(staticFS, "static")
    return "app", sub
}

func (a *App) NavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {Label: "Home", URL: "/", Position: 0},
    }
}

func (a *App) Routes(r chi.Router) {
    r.Get("/", burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
        return burrow.Render(w, r, http.StatusOK, "pages/home", map[string]any{
            "Title": "Welcome to Polls",
        })
    }))
}
```

### The Layout Name

Still in `internal/app/app.go`, add the `Layout()` function. It returns the template name for the layout:

```go
func Layout() string {
    return "app/layout"
}
```

When `Render()` is called:

1. It executes the named template (e.g. `"polls/list"`) to produce an HTML fragment
2. It checks if the request is an HTMX request — if so, it returns the fragment directly
3. Otherwise, it renders the layout template, passing the fragment as `.Content`

### The Layout Template

Create `internal/app/templates/app/layout.html`:

```html
{{ define "app/layout" -}}
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{ if .Title }}{{ .Title }} — {{ end }}Polls</title>
    <link rel="stylesheet" href="{{ staticURL "app/app.css" }}">
</head>
<body>
    <nav class="topnav">
        <ul>
            <li><a href="/" class="brand">Polls</a></li>
            {{ range navLinks -}}
            <li><a href="{{ .URL }}"{{ if .IsActive }} aria-current="page"{{ end }}>{{ .Label }}</a></li>
            {{ end -}}
        </ul>
    </nav>
    <main class="container">
        {{ .Content }}
    </main>
</body>
</html>
{{- end }}
```

`navLinks` is a built-in template function that returns the navigation items registered by all apps (via `HasNavItems`), with `IsActive` pre-computed based on the current request path. Each item has `.Label`, `.URL`, `.Icon`, and `.IsActive` fields. We mark the current page with `aria-current="page"`.

`{{ staticURL "app/app.css" }}` resolves to the content-hashed URL of `app.css` (served by the `staticfiles` contrib under our app's prefix). Create that stylesheet next.

### The Stylesheet

Create `internal/app/static/app.css` with a small hand-written rule set. The full file is in [`tutorial/step03/internal/app/static/app.css`](https://github.com/oliverandrich/burrow/blob/main/tutorial/step03/internal/app/static/app.css) — about 200 lines, covering `.topnav`, `.container`, `.btn*`, `.alert*`, `.badge*`, `.breadcrumb`, `.pagination`, a `.hero` block for the homepage, the poll-specific `.poll-*` lists, plus a few form/table defaults and `prefers-color-scheme` dark mode. Copy it as-is for now; subsequent parts reuse it unchanged.

!!! note "Why not Tailwind?"
    The tutorial keeps the styling layer deliberately tiny so it stays out of the way of teaching Burrow. For a production-grade setup using Tailwind v4 and its standalone CLI, see [Tailwind CSS](../guide/tailwind.md).

### The Homepage Template

Create `internal/app/templates/pages/home.html`:

```html
{{ define "pages/home" -}}
<section>
    <h1>Welcome to Polls</h1>
    <p>A simple polling application built with the burrow framework.</p>
    <p><a href="/polls" class="btn">View Polls &raquo;</a></p>
</section>
{{- end }}
```

`.btn` is one of the classes in our `app.css` — a basic primary-coloured button.

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
    "github.com/oliverandrich/burrow/contrib/htmx"
    "github.com/oliverandrich/burrow/contrib/staticfiles"
    "github.com/urfave/cli/v3"

    "polls/internal/app"
    "polls/internal/polls"
)

// emptyFS is used by the framework's root staticfiles app. Our project-level
// stylesheet is contributed by the `app` shell via its own HasStaticFiles.
var emptyFS embed.FS

func main() {
    staticApp, err := staticfiles.New(emptyFS)
    if err != nil {
        log.Fatal(err)
    }

    srv := burrow.NewServer(
        staticApp,
        htmx.New(),
        app.New(),
        polls.New(),
    )

    srv.SetLayout(app.Layout())

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
- **`htmx`** — provides the htmx JavaScript library
- **`app`** — homepage, layout, and the project's stylesheet
- **`polls`** — now with templates and routes

## Run It

```bash
go mod tidy
go run .
```

Open `http://localhost:8080` — you'll see the homepage. Click "View Polls" to see the (empty) polls list. There are no questions yet because we haven't added a way to create them.

!!! tip "Seeding test data"
    The polls app implements `Seedable` with a `Seed(ctx)` method that inserts a few example questions. Start the server with `--seed` once to populate the database: `go run . --seed`. The seed runs every time `--seed` is passed, so re-running it duplicates the questions — drop `data/app.db` between runs if you want a clean reset.

## What You've Learnt

- **`HasTemplates`** — apps contribute `.html` template files to the global template set
- **`Render()`** — renders a named template, automatically wrapping in a layout for normal requests and returning fragments for HTMX requests
- **Layout templates** — wrap page content in a full HTML document with navigation (via `navLinks` template function), scripts, and styles
- **`HasStaticFiles`** — apps publish their own static assets under a chosen URL prefix; the `staticfiles` contrib serves them with content-hashed URLs

## Next

In [Part 4](part4.md), you'll add a voting form with CSRF protection, flash messages, and the redirect-after-POST pattern.
