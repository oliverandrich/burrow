# Part 7: HTMX, Charts & Pagination

In this final part you'll add the `htmx` contrib app for SPA-like navigation, HTMX-powered voting, a Chart.js results visualisation, and cursor-based pagination with infinite scroll.

**Source code:** [`tutorial/step07/`](https://codeberg.org/oliverandrich/burrow/src/branch/main/tutorial/step07)

## Add the HTMX App

The `htmx` contrib app provides the htmx JavaScript library as a static asset and Go helpers for detecting HTMX requests and setting response headers.

Update `main.go`:

```go
import "codeberg.org/oliverandrich/burrow/contrib/htmx"

srv := burrow.NewServer(
    // ... existing apps ...
    htmx.New(),
    bootstrap.New(),
    // ...
)
```

Add the htmx script to the layout:

```html
<script src="{{ staticURL "htmx.min.js" }}"></script>
```

## Enable hx-boost

Add `hx-boost="true"` to the `<body>` tag in the layout:

```html
<body hx-boost="true">
```

This makes all links and forms use HTMX automatically — navigating via AJAX and swapping just the `<body>` content. Burrow's `RenderTemplate()` detects the `HX-Request` header and returns only the fragment (no layout wrapping), making this work seamlessly.

## HTMX-Aware Voting

Update the vote handler to handle both HTMX and regular requests:

```go
func (h *Handlers) Vote(w http.ResponseWriter, r *http.Request) error {
    // ... parse IDs, validate choice ...

    if err := h.repo.IncrementVotes(r.Context(), choiceID); err != nil {
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

Update `templates/polls/results.html`:

```html
{{ define "polls/results" -}}
<div class="container py-4">
    <h1>Results: {{ .Question.Text }}</h1>

    <div class="row mb-4">
        <div class="col-md-8">
            <canvas id="results-chart" height="300"></canvas>
        </div>
        <div class="col-md-4">
            <ul class="list-group">
                {{ range .Question.Choices -}}
                <li class="list-group-item d-flex justify-content-between align-items-center">
                    {{ .Text }}
                    <span class="badge text-bg-primary rounded-pill">
                        {{ .Votes }} vote{{ if ne .Votes 1 }}s{{ end }}
                    </span>
                </li>
                {{ end -}}
            </ul>
        </div>
    </div>

    <!-- ... navigation links ... -->
</div>

<script src="https://cdn.jsdelivr.net/npm/chart.js@4/dist/chart.umd.min.js"></script>
<script>
document.addEventListener("DOMContentLoaded", function() {
    const ctx = document.getElementById("results-chart");
    if (!ctx) return;
    new Chart(ctx, {
        type: "bar",
        data: {
            labels: [{{ range $i, $c := .Question.Choices }}{{ if $i }}, {{ end }}"{{ $c.Text }}"{{ end }}],
            datasets: [{
                label: "Votes",
                data: [{{ range $i, $c := .Question.Choices }}{{ if $i }}, {{ end }}{{ $c.Votes }}{{ end }}],
                backgroundColor: "rgba(13, 110, 253, 0.7)",
                borderColor: "rgb(13, 110, 253)",
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

## Cursor-Based Pagination

Replace the simple `ListQuestions` with a paginated version using Burrow's pagination helpers:

```go
func (r *Repository) ListQuestionsPaged(ctx context.Context, pr burrow.PageRequest) ([]Question, burrow.PageResult, error) {
    var questions []Question
    q := r.db.NewSelect().Model(&questions)
    q = burrow.ApplyCursor(q, pr, "id")
    if err := q.Scan(ctx); err != nil {
        return nil, burrow.PageResult{}, err
    }

    questions, hasMore := burrow.TrimCursorResults(questions, pr.Limit)
    var lastCursor string
    if len(questions) > 0 {
        lastCursor = strconv.FormatInt(questions[len(questions)-1].ID, 10)
    }
    return questions, burrow.CursorResult(lastCursor, hasMore), nil
}
```

- **`burrow.ApplyCursor()`** — adds `WHERE`, `ORDER BY`, and `LIMIT` clauses
- **`burrow.TrimCursorResults()`** — removes the extra row used to detect "has more"
- **`burrow.CursorResult()`** — builds the `PageResult` with cursor and `HasMore` flag

## Infinite Scroll

Update the list handler to detect HTMX scroll requests:

```go
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
    pr := burrow.ParsePageRequest(r)
    questions, page, err := h.repo.ListQuestionsPaged(r.Context(), pr)
    if err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list questions")
    }

    data := map[string]any{
        "Title":     "Polls",
        "Questions": questions,
        "Page":      page,
    }

    // For HTMX infinite scroll, return only the items fragment.
    if htmx.Request(r).IsHTMX() && pr.Cursor != "" {
        return burrow.RenderTemplate(w, r, http.StatusOK, "polls/list_page", data)
    }

    return burrow.RenderTemplate(w, r, http.StatusOK, "polls/list", data)
}
```

The scroll trigger in the template:

```html
{{ if .Page.HasMore -}}
<div hx-get="/polls?cursor={{ .Page.NextCursor }}&limit=20"
     hx-trigger="revealed"
     hx-target="#polls-list"
     hx-swap="beforeend">
    <div class="text-center py-3">
        <div class="spinner-border spinner-border-sm" role="status">
            <span class="visually-hidden">Loading...</span>
        </div>
    </div>
</div>
{{ end -}}
```

When the user scrolls to the bottom, htmx fetches the next page and appends the items. The `polls/list_page` template returns only the question items and a new scroll trigger (if more pages exist).

## Run It

```bash
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
- **Cursor-based pagination** — `ApplyCursor()`, `TrimCursorResults()`, `CursorResult()`
- **Infinite scroll** — `hx-trigger="revealed"` loads more items when scrolled into view

## What's Next

Congratulations — you've built a complete web application with Burrow! Here are some ideas for extending it further:

- Add i18n translations (see [i18n](../contrib/i18n.md))
- Upload images for questions (see [Uploads](../contrib/uploads.md))
- Add background jobs for vote tallying (see [Jobs](../contrib/jobs.md))
- Deploy with zero-downtime restarts (see [Deployment](../guide/deployment.md))

Explore the [Contrib Apps](../contrib/session.md) documentation for the full list of available features.
