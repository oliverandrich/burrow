# Part 7: HTMX, Charts & Pagination

In this final part you'll add the `htmx` contrib app for SPA-like navigation, HTMX-powered voting, a Chart.js results visualisation, and offset-based pagination with infinite scroll.

**Source code:** [`tutorial/step07/`](https://github.com/oliverandrich/burrow/tree/main/tutorial/step07)

## Register the htmx Contrib

The `htmx` contrib app ships the vendored `htmx.js` static asset plus Go helpers for detecting HTMX requests and setting response headers. We dropped it back in Part 4 when none of the views needed it; add it back to `main.go` now:

```go
import "github.com/oliverandrich/burrow/contrib/htmx"

srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    staticApp,
    healthcheck.New(),
    messages.New(),
    htmx.New(),           // re-added
    app.New(),
    auth.New[auth.EmptyProfile](),
    polls.New(),
    admin.New(),
)
```

## Using HTMX Helpers

In `internal/app/templates/app/layout.html`, add the htmx script and the response-config helper inside `<head>`, and add `hx-boost="true"` plus the CSRF-headers helper to the `<body>` tag:

```html
{{ template "htmx/js" . }}
{{ template "htmx/config" . }}
```

```html
<body hx-boost="true" {{- csrfHxHeaders }}>
```

`hx-boost` makes all links and forms use HTMX automatically — navigating via AJAX and swapping just the `<body>` content. Burrow's `Render()` detects the `HX-Request` header and returns only the fragment (no layout wrapping), making this work seamlessly. `csrfHxHeaders` (provided by the `csrf` contrib) renders the htmx config attribute that injects the CSRF token into every HTMX request — so we no longer need to embed `{{ csrfField }}` inside every form. `htmx/config` adds a small `<meta>` tag that tells htmx to swap `422` responses too (used by form-validation error responses later).

## Sign out via HTMX

The inline `<form>` logout button from Part 5 still works — `hx-boost` automatically AJAX-ifies it. No change needed in markup. The button picks up the CSRF token via `csrfHxHeaders` (no per-form `csrfField` required when posting from inside a boosted body).

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
    resultsURL := fmt.Sprintf("/polls/%s/results", questionID)

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
    const primary = getComputedStyle(document.documentElement).getPropertyValue("--accent").trim() || "#1095c1";
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
    ptrs, count, err := den.NewQuery[Question](r.db).
        Sort("_id", den.Desc).
        Limit(pr.Limit).
        Skip(pr.Offset()).
        AllWithCount(ctx)
    if err != nil {
        return nil, burrow.PageResult{}, fmt.Errorf("list questions paged: %w", err)
    }

    questions := make([]Question, len(ptrs))
    for i, p := range ptrs {
        questions[i] = *p
    }
    return questions, burrow.OffsetResult(pr, int(count)), nil
}
```

- **`.Limit()` / `.Skip()`** — chainable methods for pagination
- **`.AllWithCount()`** — returns the page slice and the total count in a single round-trip (no separate `Count` query)
- **`Sort("_id", ...)`** — Den's internal ULID-based primary key; sorting on it gives a stable newest-first order without needing a separate index
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

## Next

In [Part 8](part8.md) you'll add custom admin views for managing polls — implementing `HasAdmin` on the polls app itself so administrators can create, edit, and delete questions and choices from the admin panel.
