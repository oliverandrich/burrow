# Part 7: HTMX, Charts & Pagination

In this final part you'll add the `htmx` contrib app for SPA-like navigation, HTMX-powered voting, a Chart.js results visualisation, and offset-based pagination with infinite scroll.

**Source code:** [`tutorial/step07/`](https://github.com/oliverandrich/burrow/tree/main/tutorial/step07)

## Using HTMX Helpers

The `htmx` contrib app (registered since Part 3) provides Go helpers for detecting HTMX requests and setting response headers. To use htmx on the client side, add the script to your layout.

In `internal/pages/templates/app/layout.html`, add the htmx script in the `<head>` alongside the existing `mucss/css` and `mucss/theme_script` templates, and add `hx-boost="true"` plus the CSRF-headers helper to the `<body>` tag:

```html
{{ template "htmx/js" . }}
```

```html
<body hx-boost="true" {{- csrfHxHeaders }}>
```

`hx-boost` makes all links and forms use HTMX automatically — navigating via AJAX and swapping just the `<body>` content. Burrow's `Render()` detects the `HX-Request` header and returns only the fragment (no layout wrapping), making this work seamlessly. `csrfHxHeaders` (provided by the `csrf` contrib) renders the htmx config attribute that injects the CSRF token into every HTMX request — so we no longer need to embed `{{ csrfField }}` inside every form.

## Polish the User Menu

Now that htmx is loaded, the inline-`<form>` logout button from Part 5 can become a proper dropdown menu with a `hx-post` link. **Replace** the entire `{{ if currentUser -}} … {{ end -}}` block in the right `<ul>` of the navbar (the inline form + `<style>` block in `<head>` for `nav .logout-form`) with this:

```html
<ul>
    {{ if currentUser -}}
    <li>
        <details class="dropdown">
            <summary>{{ (currentUser).Username }}</summary>
            <ul dir="rtl">
                <li dir="ltr"><a href="#" hx-post="/auth/logout">Sign out</a></li>
            </ul>
        </details>
    </li>
    {{ else -}}
    <li><a href="/auth/login">Sign in</a></li>
    {{ end -}}
    <li>{{ template "mucss/theme_switcher" . }}</li>
</ul>
```

The `<details class="dropdown">` is a native HTML5 disclosure widget that µCSS styles as a menu. The "Sign out" link triggers a POST via `hx-post` — htmx serializes no body, but the CSRF token comes through via the `csrfHxHeaders` we added to the `<body>`. `<ul dir="rtl">` right-anchors the popup; `<li dir="ltr">` keeps each item's text in normal reading order.

You can now drop the `<style>` block with `nav .logout-form` rules — the dropdown doesn't need it.

## HTMX-Aware Voting

In `internal/polls/polls.go`, add the `htmx` import and update the `Vote` handler to handle both HTMX and regular requests:

```go
"github.com/oliverandrich/burrow/contrib/htmx"
```

```go
func (a *App) Vote(w http.ResponseWriter, r *http.Request) error {
    // ... parse IDs, validate choice ...

    if err := a.repo.IncrementVotes(r.Context(), choiceID); err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to record vote")
    }

    _ = messages.AddSuccess(w, r, "Your vote has been recorded!")
    resultsURL := fmt.Sprintf("/polls/%d/results", questionID)

    if htmx.Request(r).IsHTMX() {
        htmx.Redirect(w, resultsURL)
        return nil
    }
    http.Redirect(w, r, resultsURL, http.StatusSeeOther)
    return nil
}
```

- **`htmx.Request(r).IsHTMX()`** — checks for the `HX-Request` header
- **`htmx.Redirect(w, url)`** — sets the `HX-Redirect` header, telling htmx to navigate to the URL

## Results Chart with Chart.js

Add a bar chart to the results page using [Chart.js](https://www.chartjs.org/) loaded from a CDN. The chart shows vote counts per choice as a horizontal bar chart alongside the existing badge list.

Update `internal/polls/templates/polls/results.html`:

```html
{{ define "polls/results" -}}
<header>
    <h1>Results: {{ .Question.Text }}</h1>
</header>

<div class="grid">
    <div>
        <canvas id="results-chart" height="300"></canvas>
    </div>
    <div>
        <ul class="poll-results">
            {{ range .Question.Choices -}}
            <li>
                <span>{{ .Text }}</span>
                <span class="badge badge-primary">{{ .Votes }} vote{{ if ne .Votes 1 }}s{{ end }}</span>
            </li>
            {{ end -}}
        </ul>
    </div>
</div>

<!-- ... navigation links ... -->

<script src="https://cdn.jsdelivr.net/npm/chart.js@4/dist/chart.umd.min.js"></script>
<script>
document.addEventListener("DOMContentLoaded", function() {
    const ctx = document.getElementById("results-chart");
    if (!ctx) return;
    const primary = getComputedStyle(document.documentElement).getPropertyValue("--mu-primary").trim() || "#1095c1";
    new Chart(ctx, {
        type: "bar",
        data: {
            labels: [{{ range $i, $c := .Question.Choices }}{{ if $i }}, {{ end }}"{{ $c.Text }}"{{ end }}],
            datasets: [{
                label: "Votes",
                data: [{{ range $i, $c := .Question.Choices }}{{ if $i }}, {{ end }}{{ $c.Votes }}{{ end }}],
                backgroundColor: primary,
                borderColor: primary,
                borderWidth: 1,
                borderRadius: 4
            }]
        },
        options: {
            responsive: true,
            indexAxis: "y",
            scales: { x: { beginAtZero: true, ticks: { stepSize: 1 } } },
            plugins: { legend: { display: false } }
        }
    });
});
</script>
{{- end }}
```

Key points:

- **CDN loading** — Chart.js is loaded from jsDelivr, keeping it simple (no bundler needed)
- **`indexAxis: "y"`** — renders horizontal bars, which are easier to read for text labels
- **Go template loops** — the `{{ range }}` blocks generate the JavaScript arrays server-side
- **`DOMContentLoaded`** — ensures the canvas element exists before Chart.js initialises

## Offset-Based Pagination

In `internal/polls/polls.go`, replace the simple `ListQuestions` with a paginated version using Burrow's pagination helpers:

```go
func (r *Repository) ListQuestionsPaged(ctx context.Context, pr burrow.PageRequest) ([]Question, burrow.PageResult, error) {
    count, err := den.NewQuery[Question](r.db).Count(ctx)
    if err != nil {
        return nil, burrow.PageResult{}, err
    }

    questions, err := den.NewQuery[Question](r.db).
        Sort("id", den.Desc).
        Limit(pr.Limit).
        Skip(pr.Offset()).
        All(ctx)
    if err != nil {
        return nil, burrow.PageResult{}, err
    }

    return questions, burrow.OffsetResult(pr, count), nil
}
```

- **`.Limit()` / `.Skip()`** — chainable methods for pagination
- **`burrow.OffsetResult()`** — builds the `PageResult` with page numbers and `HasMore` flag

## Infinite Scroll

Still in `internal/polls/polls.go`, update the `List` handler to detect HTMX scroll requests:

```go
func (a *App) List(w http.ResponseWriter, r *http.Request) error {
    pr := burrow.ParsePageRequest(r)
    questions, page, err := a.repo.ListQuestionsPaged(r.Context(), pr)
    if err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list questions")
    }

    data := map[string]any{
        "Title":     "Polls",
        "Questions": questions,
        "Page":      page,
    }

    // For HTMX infinite scroll, return only the items fragment.
    if htmx.Request(r).IsHTMX() && pr.Page > 1 {
        return burrow.Render(w, r, http.StatusOK, "polls/list_page", data)
    }

    return burrow.Render(w, r, http.StatusOK, "polls/list", data)
}
```

Extract the question item into its own template so both the initial list and the scroll-loaded chunks can render it identically. Create `internal/polls/templates/polls/question_item.html`:

```html
{{ define "polls/question_item" -}}
<a href="/polls/{{ .ID }}" class="polls-list-item">
    <article>
        <strong>{{ .Text }}</strong>
        <small>{{ .PublishedAt.Format "2 Jan 2006" }}</small>
    </article>
</a>
{{- end }}
```

Update `internal/polls/templates/polls/list.html` to add an `id` to the list container and append the scroll trigger:

```html
{{ define "polls/list" -}}
<header>
    <h1>Polls</h1>
</header>
{{ if .Questions -}}
<div class="polls-list" id="polls-list">
    {{ range .Questions -}}
    {{ template "polls/question_item" . }}
    {{ end -}}
</div>
{{ if .Page.HasMore -}}
<div hx-get="/polls?page={{ add .Page.Page 1 }}&limit=20"
     hx-trigger="revealed"
     hx-target="#polls-list"
     hx-swap="beforeend"
     hx-select="template">
    <p aria-busy="true">Loading…</p>
</div>
{{ end -}}
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

Create a new file `internal/polls/templates/polls/list_page.html` — it returns only the question items and a new scroll trigger (no layout wrapping):

```html
{{ define "polls/list_page" -}}
{{ range .Questions -}}
{{ template "polls/question_item" . }}
{{ end -}}
{{ if .Page.HasMore -}}
<div hx-get="/polls?page={{ add .Page.Page 1 }}&limit=20"
     hx-trigger="revealed"
     hx-target="#polls-list"
     hx-swap="beforeend">
    <p aria-busy="true">Loading…</p>
</div>
{{ end -}}
{{- end }}
```

When the user scrolls to the bottom, htmx fetches the next page and appends the items.

## Run It

```bash
go mod tidy
go run .
```

The application now has:

- Smooth page transitions via `hx-boost` (no full page reloads)
- HTMX-powered voting with `HX-Redirect`
- A Chart.js bar chart on the results page
- Infinite scroll on the question list

## What You've Learnt

- **`htmx.New()`** — provides the htmx JavaScript library as a static asset
- **`htmx.Request(r).IsHTMX()`** — detects HTMX requests for conditional logic
- **`htmx.Redirect()`** — client-side redirect via response header
- **`hx-boost`** — automatic AJAX navigation with history management
- **Chart.js** — CDN-loaded charting library with server-rendered data via Go templates
- **Offset-based pagination** — `.Limit()`, `.Skip()`, `OffsetResult()`
- **Infinite scroll** — `hx-trigger="revealed"` loads more items when scrolled into view

## What's Next

Congratulations — you've built a complete web application with Burrow! Here are some ideas for extending it further:

- Add i18n translations (see [i18n](../guide/i18n.md))
- Upload images for questions (see [Uploader](../guide/uploader.md))
- Add background jobs for vote tallying (see [Jobs](../contrib/jobs.md))
- Deploy with zero-downtime restarts (see [Deployment](../guide/deployment.md))

Explore the [Contrib Apps](../contrib/session.md) documentation for the full list of available features.
