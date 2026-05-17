# Part 8: Custom Admin Views

In [Part 6](part6.md) you registered the `admin` contrib app and got the auth and jobs admin views for free. The polls app itself, however, didn't contribute anything to the admin panel — administrators could manage users but not the questions and choices that make up the application.

In this final part you'll implement `burrow.HasAdmin` on the polls app to add custom admin views for managing polls.

**Source code:** [`tutorial/step08/`](https://github.com/oliverandrich/burrow/tree/main/tutorial/step08)

## The `HasAdmin` Interface

Apps contribute admin pages by implementing two methods:

```go
type HasAdmin interface {
    AdminRoutes(r chi.Router)
    AdminNavItems() []NavItem
}
```

- **`AdminRoutes`** — receives a chi router already prefixed with `/admin` and protected by the admin auth middleware. Mount your admin routes here.
- **`AdminNavItems`** — returns nav items shown on the admin dashboard. The admin coordinator collects one nav group per `HasAdmin` app.

## Add Repository Methods

Admin views need a few extra repository methods. In `internal/polls/polls.go`, add these alongside the existing ones:

```go
// GetQuestionByID fetches a question without its choices (single query).
// Use this in admin handlers that only need to verify existence or update
// the question text — they don't need to pay for the second choices query.
func (r *Repository) GetQuestionByID(ctx context.Context, id string) (*Question, error) {
    question, err := den.FindByID[Question](ctx, r.db, id)
    if err != nil {
        return nil, fmt.Errorf("get question %s: %w", id, err)
    }
    return question, nil
}

// SearchQuestionsPaged searches questions by their text with pagination.
func (r *Repository) SearchQuestionsPaged(ctx context.Context, query string, pr burrow.PageRequest) ([]Question, burrow.PageResult, error) {
    ptrs, count, err := den.NewQuery[Question](r.db, where.Field("text").StringContains(query)).
        Sort("_id", den.Desc).
        Limit(pr.Limit).
        Skip(pr.Offset()).
        AllWithCount(ctx)
    if err != nil {
        return nil, burrow.PageResult{}, fmt.Errorf("search questions: %w", err)
    }
    questions := make([]Question, len(ptrs))
    for i, p := range ptrs {
        questions[i] = *p
    }
    return questions, burrow.OffsetResult(pr, int(count)), nil
}

// UpdateQuestion saves changes to an existing question.
func (r *Repository) UpdateQuestion(ctx context.Context, q *Question) error {
    return den.Save(ctx, r.db, q)
}

// DeleteQuestion removes a question and all its choices.
func (r *Repository) DeleteQuestion(ctx context.Context, id string) error {
    if _, err := den.NewQuery[Choice](r.db, where.Field("question_id").Eq(id)).Delete(ctx); err != nil {
        return fmt.Errorf("delete choices for %s: %w", id, err)
    }
    q, err := r.GetQuestionByID(ctx, id)
    if err != nil {
        return err
    }
    return den.Delete(ctx, r.db, q)
}

// UpdateChoice saves changes to an existing choice.
func (r *Repository) UpdateChoice(ctx context.Context, c *Choice) error {
    return den.Save(ctx, r.db, c)
}

// DeleteChoice removes a single choice.
func (r *Repository) DeleteChoice(ctx context.Context, id string) error {
    c, err := den.FindByID[Choice](ctx, r.db, id)
    if err != nil {
        return fmt.Errorf("find choice %s: %w", id, err)
    }
    return den.Delete(ctx, r.db, c)
}

// GetChoice fetches a choice by id.
func (r *Repository) GetChoice(ctx context.Context, id string) (*Choice, error) {
    c, err := den.FindByID[Choice](ctx, r.db, id)
    if err != nil {
        return nil, fmt.Errorf("get choice %s: %w", id, err)
    }
    return c, nil
}
```

`DeleteQuestion` deletes the choices first, then the question itself. Den's `WithLinkRule(LinkDelete)` cascade only follows typed `Link[T]` fields on the parent — our `Question` and `Choice` are joined by a plain foreign-key field (`Choice.QuestionID`), which is the back-link / one-to-many pattern, so the cascade has to be manual via `DeleteMany[Choice]`. (If `Question` instead held a `Choices []Link[Choice]` field, `WithLinkRule(LinkDelete)` would cascade automatically.) `where.Field("text").StringContains(query)` does a case-insensitive substring match on the JSON document. `GetQuestionByID` is a deliberately slim variant of the existing `GetQuestion`: handlers that only need to verify existence or update the question text shouldn't pay for the second query that loads all choices.

## Implement `HasAdmin`

Still in `internal/polls/polls.go`, add the two interface methods:

```go
// AdminNavItems contributes one nav item to the admin dashboard.
func (a *App) AdminNavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {Label: "Polls", URL: "/admin/polls", Position: 50},
    }
}

// AdminRoutes mounts the admin views under /admin/polls.
func (a *App) AdminRoutes(r chi.Router) {
    r.Route("/polls", func(r chi.Router) {
        r.Get("/", burrow.Handle(a.adminList))
        r.Get("/new", burrow.Handle(a.adminNew))
        r.Post("/", burrow.Handle(a.adminCreate))
        r.Get("/{id}", burrow.Handle(a.adminEdit))
        r.Post("/{id}", burrow.Handle(a.adminUpdate))
        r.Delete("/{id}", burrow.Handle(a.adminDelete))
        r.Post("/{id}/choices", burrow.Handle(a.adminAddChoice))
        r.Post("/{id}/choices/{choiceID}", burrow.Handle(a.adminUpdateChoice))
        r.Delete("/{id}/choices/{choiceID}", burrow.Handle(a.adminDeleteChoice))
    })
}
```

Note that you don't need `r.Use(auth.RequireAdmin())` here — the admin coordinator already wrapped the router with `RequireAuth()` and `RequireAdmin()` before calling `AdminRoutes`.

## List Handler

The list handler mirrors the public `List` handler but adds search and uses an admin-flavoured template:

```go
func (a *App) adminList(w http.ResponseWriter, r *http.Request) error {
    pr := burrow.ParsePageRequest(r)
    searchTerm := r.URL.Query().Get("q")

    var (
        questions []Question
        page      burrow.PageResult
        err       error
    )
    if searchTerm != "" {
        questions, page, err = a.repo.SearchQuestionsPaged(r.Context(), searchTerm, pr)
    } else {
        questions, page, err = a.repo.ListQuestionsPaged(r.Context(), pr)
    }
    if err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list questions")
    }

    return burrow.Render(w, r, http.StatusOK, "polls/admin_list", map[string]any{
        "Title":      "Manage Polls",
        "Questions":  questions,
        "Page":       page,
        "SearchTerm": searchTerm,
        "RawQuery":   r.URL.RawQuery,
    })
}
```

`RawQuery` is passed to the pagination template so search and filter parameters are preserved across page links.

## Distinguishing Not-Found from Server Errors

Several admin handlers below need to surface 404 only when the requested document doesn't exist, and 500 on any other repository error (DB outage, disk error, …). Without that split, a transient infrastructure failure would silently appear as 404 to the user. Add a small helper at the top of `polls.go`:

```go
import "errors"

// notFoundOrServerError discriminates between "no such row" (HTTP 404)
// and any other repository error (HTTP 500). The repository wraps
// den.ErrNotFound via %w so errors.Is traverses the chain to find it.
func notFoundOrServerError(err error, notFoundMsg, serverMsg string) error {
    if errors.Is(err, den.ErrNotFound) {
        return burrow.NewHTTPError(http.StatusNotFound, notFoundMsg)
    }
    return burrow.NewHTTPError(http.StatusInternalServerError, serverMsg)
}
```

## Create / Edit / Delete Handlers

```go
func (a *App) adminNew(w http.ResponseWriter, r *http.Request) error {
    return burrow.Render(w, r, http.StatusOK, "polls/admin_form", map[string]any{
        "Title":    "New Poll",
        "Question": &Question{PublishedAt: time.Now()},
        "IsNew":    true,
    })
}

func (a *App) adminCreate(w http.ResponseWriter, r *http.Request) error {
    r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

    text := strings.TrimSpace(r.FormValue("text"))
    if text == "" {
        return burrow.NewHTTPError(http.StatusUnprocessableEntity, "question text is required")
    }

    q := &Question{Text: text, PublishedAt: time.Now()}
    if err := a.repo.CreateQuestion(r.Context(), q); err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to create question")
    }

    for _, ct := range r.Form["choice"] {
        ct = strings.TrimSpace(ct)
        if ct == "" {
            continue
        }
        if err := a.repo.CreateChoice(r.Context(), &Choice{QuestionID: q.ID, Text: ct}); err != nil {
            return burrow.NewHTTPError(http.StatusInternalServerError, "failed to create choice")
        }
    }

    _ = messages.AddSuccess(w, r, "Poll created.")
    htmx.SmartRedirect(w, r, "/admin/polls/"+q.ID)
    return nil
}
```

`r.Form["choice"]` returns all values for the repeated `name="choice"` inputs in the create form. Empty entries are skipped so the user can leave unused slots blank.

```go
func (a *App) adminEdit(w http.ResponseWriter, r *http.Request) error {
    id := chi.URLParam(r, "id")
    question, err := a.repo.GetQuestion(r.Context(), id)
    if err != nil {
        return notFoundOrServerError(err, "question not found", "failed to load question")
    }
    return burrow.Render(w, r, http.StatusOK, "polls/admin_form", map[string]any{
        "Title":    "Edit Poll",
        "Question": question,
        "IsNew":    false,
    })
}

func (a *App) adminUpdate(w http.ResponseWriter, r *http.Request) error {
    r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

    id := chi.URLParam(r, "id")
    question, err := a.repo.GetQuestionByID(r.Context(), id)
    if err != nil {
        return notFoundOrServerError(err, "question not found", "failed to load question")
    }

    text := strings.TrimSpace(r.FormValue("text"))
    if text == "" {
        return burrow.NewHTTPError(http.StatusUnprocessableEntity, "question text is required")
    }
    question.Text = text
    if err := a.repo.UpdateQuestion(r.Context(), question); err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to update question")
    }

    _ = messages.AddSuccess(w, r, "Poll saved.")
    htmx.SmartRedirect(w, r, "/admin/polls/"+id)
    return nil
}

func (a *App) adminDelete(w http.ResponseWriter, r *http.Request) error {
    id := chi.URLParam(r, "id")
    if err := a.repo.DeleteQuestion(r.Context(), id); err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to delete question")
    }
    _ = messages.AddSuccess(w, r, "Poll deleted.")
    htmx.SmartRedirect(w, r, "/admin/polls")
    return nil
}
```

`htmx.SmartRedirect` issues an `HX-Redirect` header for HTMX requests and a plain HTTP redirect otherwise.

## Choice Handlers

The edit page also lets administrators add, rename, and delete individual choices:

```go
func (a *App) adminAddChoice(w http.ResponseWriter, r *http.Request) error {
    r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

    id := chi.URLParam(r, "id")
    if _, err := a.repo.GetQuestionByID(r.Context(), id); err != nil {
        return notFoundOrServerError(err, "question not found", "failed to load question")
    }

    text := strings.TrimSpace(r.FormValue("text"))
    if text == "" {
        return burrow.NewHTTPError(http.StatusUnprocessableEntity, "choice text is required")
    }
    if err := a.repo.CreateChoice(r.Context(), &Choice{QuestionID: id, Text: text}); err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to add choice")
    }

    htmx.SmartRedirect(w, r, "/admin/polls/"+id)
    return nil
}

func (a *App) adminUpdateChoice(w http.ResponseWriter, r *http.Request) error {
    r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

    id := chi.URLParam(r, "id")
    choiceID := chi.URLParam(r, "choiceID")
    choice, err := a.repo.GetChoice(r.Context(), choiceID)
    if err != nil {
        return notFoundOrServerError(err, "choice not found", "failed to load choice")
    }

    text := strings.TrimSpace(r.FormValue("text"))
    if text == "" {
        return burrow.NewHTTPError(http.StatusUnprocessableEntity, "choice text is required")
    }
    choice.Text = text
    if err := a.repo.UpdateChoice(r.Context(), choice); err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to update choice")
    }

    htmx.SmartRedirect(w, r, "/admin/polls/"+id)
    return nil
}

func (a *App) adminDeleteChoice(w http.ResponseWriter, r *http.Request) error {
    id := chi.URLParam(r, "id")
    choiceID := chi.URLParam(r, "choiceID")
    if err := a.repo.DeleteChoice(r.Context(), choiceID); err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to delete choice")
    }
    htmx.SmartRedirect(w, r, "/admin/polls/"+id)
    return nil
}
```

## Templates

Create `internal/polls/templates/polls/admin_list.html`. The file defines both the list page and a small `polls/pagination` partial it uses at the bottom:

```html
{{ define "polls/admin_list" -}}
<nav>
    <ul class="breadcrumb" aria-label="breadcrumb">
        <li><a href="/admin/">Admin</a></li>
        <li aria-current="page">Polls</li>
    </ul>
</nav>
<header>
    <h2>Polls</h2>
    <form action="/admin/polls" method="GET">
        <input type="search" name="q" value="{{ .SearchTerm }}" placeholder="Search…" aria-label="Search">
        <a href="/admin/polls/new" class="btn">New poll</a>
    </form>
</header>
{{- if .Questions }}
<table>
    <thead>
        <tr>
            <th scope="col">Question</th>
            <th scope="col">Published</th>
            <th scope="col">Actions</th>
        </tr>
    </thead>
    <tbody>
        {{- range .Questions }}
        <tr>
            <td><a href="/admin/polls/{{ .ID }}">{{ .Text }}</a></td>
            <td>{{ .PublishedAt.Format "2006-01-02 15:04" }}</td>
            <td>
                <a href="/admin/polls/{{ .ID }}" class="btn btn-sm btn-outline">Edit</a>
                <button type="button" class="btn btn-sm btn-error"
                    hx-delete="/admin/polls/{{ .ID }}"
                    hx-swap="none"
                    hx-confirm="Delete this poll and all its choices?">Delete</button>
            </td>
        </tr>
        {{- end }}
    </tbody>
</table>
{{ template "polls/pagination" (dict "Page" .Page "BasePath" "/admin/polls" "RawQuery" .RawQuery) }}
{{- else }}
<p>No polls yet.</p>
{{- end }}
{{- end }}
```

The pagination partial — a thin wrapper around the `pageURL` and `pageNumbers` framework helpers, styled by our `.pagination` rules in `app.css` — see [`tutorial/step08/internal/polls/templates/polls/admin_list.html`](https://github.com/oliverandrich/burrow/blob/main/tutorial/step08/internal/polls/templates/polls/admin_list.html) for the full `polls/pagination` define.

!!! tip "Reuse `admin/pagination` when you ship Tailwind"
    Burrow's `contrib/admin` ships a ready-made `admin/pagination` template that uses Tailwind classes. Once your project's stylesheet includes those classes (see the [Tailwind guide](../guide/tailwind.md)), you can drop the local `polls/pagination` define and call `{{ template "admin/pagination" ... }}` instead — same data contract.

Key bits:

- The breadcrumb pattern (`<nav><ul class="breadcrumb">`) sits above a `<header>` with the page title and a search/new-poll row — the `.breadcrumb` class is one of the rules in our `app.css`.
- `hx-delete` + `hx-confirm` give a one-click delete with a browser confirmation, no separate confirmation page needed.
- Passing `RawQuery` to `polls/pagination` keeps the search term in the URLs.

Create `internal/polls/templates/polls/admin_form.html`:

```html
{{ define "polls/admin_form" -}}
<nav>
    <ul class="breadcrumb" aria-label="breadcrumb">
        <li><a href="/admin/">Admin</a></li>
        <li><a href="/admin/polls">Polls</a></li>
        <li aria-current="page">{{ if .IsNew }}New{{ else }}{{ .Question.Text }}{{ end }}</li>
    </ul>
</nav>
<header>
    <h2>{{ if .IsNew }}New poll{{ else }}Edit poll{{ end }}</h2>
</header>

<form method="post" action="{{ if .IsNew }}/admin/polls{{ else }}/admin/polls/{{ .Question.ID }}{{ end }}">
    {{ csrfField }}
    <label>
        Question text *
        <input type="text" name="text" value="{{ .Question.Text }}" required autofocus>
    </label>
    {{- if .IsNew }}
    <fieldset>
        <legend>Initial choices</legend>
        <p><small>Leave blank to skip. You can add and edit choices after creating the poll.</small></p>
        <input type="text" name="choice" placeholder="Choice 1">
        <input type="text" name="choice" placeholder="Choice 2">
        <input type="text" name="choice" placeholder="Choice 3">
        <input type="text" name="choice" placeholder="Choice 4">
    </fieldset>
    {{- end }}
    <footer class="admin-row-actions">
        <button type="submit" class="btn btn-primary">{{ if .IsNew }}Create{{ else }}Save{{ end }}</button>
        <a href="/admin/polls" role="button" class="btn btn-outline btn-secondary">Back</a>
    </footer>
</form>

{{- if not .IsNew }}
<header>
    <h3>Choices</h3>
</header>
{{- if .Question.Choices }}
<ul class="poll-admin-choices">
    {{- range .Question.Choices }}
    <li>
        <form method="post" action="/admin/polls/{{ $.Question.ID }}/choices/{{ .ID }}">
            {{ csrfField }}
            <fieldset role="group">
                <input type="text" name="text" value="{{ .Text }}" required>
                <span class="badge badge-secondary">{{ .Votes }} votes</span>
                <button type="submit" class="btn btn-primary">Save</button>
                <button type="button" class="btn btn-outline btn-error"
                    hx-delete="/admin/polls/{{ $.Question.ID }}/choices/{{ .ID }}"
                    hx-swap="none"
                    hx-confirm="Delete this choice?">Delete</button>
            </fieldset>
        </form>
    </li>
    {{- end }}
</ul>
{{- else }}
<p>No choices yet.</p>
{{- end }}

<form method="post" action="/admin/polls/{{ .Question.ID }}/choices">
    {{ csrfField }}
    <label>
        Add choice
        <input type="text" name="text" placeholder="New choice text" required>
    </label>
    <button type="submit" class="btn btn-primary">Add choice</button>
</form>
<style>
.poll-admin-choices{list-style:none;padding:0;display:flex;flex-direction:column;gap:.5rem}
.poll-admin-choices > li{margin:0}
.poll-admin-choices form{margin:0}
</style>
{{- end }}
{{- end }}
```

Two things worth noting:

- Each existing-choice row is its own `<form>` so the Save button only submits that row's text. The Delete button inside the same form uses `hx-delete` and bypasses the form submission — htmx fires the DELETE directly without a body.
- `$.Question.ID` reaches up out of the inner `range` to the outer template scope.

## Run It

```bash
go mod tidy
go run .
```

Sign in as your admin user and visit `/admin/`. The dashboard now shows a **Polls** card alongside the existing **Auth** and **Jobs** sections. Clicking it lands on the question list with search, pagination, and delete actions; the **New poll** button opens the create form, and clicking a question opens the edit page with its choices.

## What You've Learnt

- **`burrow.HasAdmin`** — two-method interface for contributing admin routes and nav items
- **Admin routes are pre-wrapped** — the `/admin` prefix and auth middleware are applied for you
- **Repository methods for admin** — search, update, and delete operations to round out the read-mostly public surface
- **Cascade delete for back-link relationships** — manual two-step delete (`DeleteMany` children, then delete parent), because Den's `WithLinkRule(LinkDelete)` cascade only follows typed `Link[T]` fields on the parent, not foreign-key fields on the children
- **`htmx.SmartRedirect`** — single helper that picks `HX-Redirect` or `Location` based on the request type
- **One form per row** — a clean pattern for editable lists without dedicated edit pages

## Where to Go Next

You've now built and extended a complete Burrow application from scratch. Some directions worth exploring:

- Add i18n translations (see [i18n](../guide/i18n.md))
- Upload images for questions (see [Uploader](../guide/uploader.md))
- Add background jobs for tally aggregation (see [Jobs](../contrib/jobs.md))
- Deploy with zero-downtime restarts (see [Deployment](../guide/deployment.md))

Browse the [Contrib Apps](../contrib/session.md) documentation for the full list of available features.
