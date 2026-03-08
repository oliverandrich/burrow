# Part 4: Forms, CRUD & Validation

In this part you'll add a voting form with CSRF protection, flash messages, and the redirect-after-POST pattern.

**Source code:** [`tutorial/step04/`](https://codeberg.org/oliverandrich/burrow/src/branch/main/tutorial/step04)

## New Contrib Apps

This step introduces two new contrib apps:

- **`csrf`** — CSRF protection via gorilla/csrf. Injects a `csrfToken` template function.
- **`messages`** — Flash messages that survive redirects. Stored in the session.

Update `main.go` — add the new imports and apps:

```go
import (
    "codeberg.org/oliverandrich/burrow/contrib/csrf"
    "codeberg.org/oliverandrich/burrow/contrib/messages"
    "codeberg.org/oliverandrich/burrow/contrib/session"
)
```

Then update the `NewServer` call:

```go
srv := burrow.NewServer(
    session.New(),
    csrf.New(),          // new
    staticApp,
    htmx.New(),
    messages.New(),      // new
    bootstrap.New(),
    pages.New(),
    polls.New(),
)
```

## Add a Voting Form

Update the detail template to include a form with radio buttons:

```html
{{ define "polls/detail" -}}
<div class="container py-4">
    <h1>{{ .Question.Text }}</h1>
    <form method="post" action="/polls/{{ .Question.ID }}/vote">
        <input type="hidden" name="gorilla.csrf.Token" value="{{ csrfToken }}">
        <div class="list-group mb-3">
            {{ range .Question.Choices -}}
            <label class="list-group-item">
                <input class="form-check-input me-2" type="radio"
                       name="choice" value="{{ .ID }}">
                {{ .Text }}
            </label>
            {{ end -}}
        </div>
        <button type="submit" class="btn btn-primary">Vote</button>
        <a href="/polls" class="btn btn-secondary">&laquo; Back to polls</a>
    </form>
</div>
{{- end }}
```

Key points:

- **`{{ csrfToken }}`** is a template function provided by the `csrf` app via `HasRequestFuncMap`. It returns the CSRF token for the current request.
- The token is submitted as a hidden form field named `gorilla.csrf.Token`.
- Without a valid token, the POST request will be rejected with a 403.

## Handle the Vote

First, add the `messages` import to `internal/polls/polls.go`:

```go
"codeberg.org/oliverandrich/burrow/contrib/messages"
```

Add the `IncrementVotes` method to the repository:

```go
func (r *Repository) IncrementVotes(ctx context.Context, choiceID int64) error {
    _, err := r.db.NewUpdate().
        Model((*Choice)(nil)).
        Set("votes = votes + 1").
        Where("id = ?", choiceID).
        Exec(ctx)
    return err
}
```

Then add a `Vote` handler:

```go
func (h *Handlers) Vote(w http.ResponseWriter, r *http.Request) error {
    questionID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
    if err != nil {
        return burrow.NewHTTPError(http.StatusBadRequest, "invalid question ID")
    }

    choiceIDStr := r.FormValue("choice")
    if choiceIDStr == "" {
        _ = messages.AddError(w, r, "You didn't select a choice.")
        http.Redirect(w, r, fmt.Sprintf("/polls/%d", questionID), http.StatusSeeOther)
        return nil
    }

    choiceID, err := strconv.ParseInt(choiceIDStr, 10, 64)
    if err != nil {
        return burrow.NewHTTPError(http.StatusBadRequest, "invalid choice ID")
    }

    if err := h.repo.IncrementVotes(r.Context(), choiceID); err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to record vote")
    }

    _ = messages.AddSuccess(w, r, "Your vote has been recorded!")
    http.Redirect(w, r, fmt.Sprintf("/polls/%d/results", questionID), http.StatusSeeOther)
    return nil
}
```

This demonstrates:

- **`r.FormValue()`** — reads form values from the POST body
- **`messages.AddError()`** / **`messages.AddSuccess()`** — flash messages stored in the session
- **Redirect-after-POST** — `http.StatusSeeOther` (303) prevents double submission on refresh

Register the route:

```go
func (a *App) Routes(r chi.Router) {
    r.Route("/polls", func(r chi.Router) {
        r.Get("/", burrow.Handle(a.handlers.List))
        r.Get("/{id}", burrow.Handle(a.handlers.Detail))
        r.Post("/{id}/vote", burrow.Handle(a.handlers.Vote))  // new
        r.Get("/{id}/results", burrow.Handle(a.handlers.Results))
    })
}
```

## Display Flash Messages

Update the layout to show messages above the content:

```html
<main class="container">
    {{ if .Messages -}}
    {{ range .Messages -}}
    <div class="alert alert-{{ .Level }} alert-dismissible fade show" role="alert">
        {{ .Text }}
        <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
    </div>
    {{ end -}}
    {{ end -}}
    {{ .Content }}
</main>
```

In `internal/pages/pages.go`, add the `messages` import:

```go
"codeberg.org/oliverandrich/burrow/contrib/messages"
```

Then update the layout function to pass messages to the template:

```go
layoutData := map[string]any{
    "Content":  content,
    "NavItems": burrow.NavItems(r.Context()),
    "Messages": messages.Get(r.Context()),  // new
}
```

Messages have a `Level` (success, error, warning, info) and `Text`. Each level maps naturally to a Bootstrap alert class.

## Run It

```bash
go run .
```

Seed some test data, then navigate to a question. Select a choice and click "Vote" — you'll be redirected to the results page with a success message. Try submitting without selecting a choice to see the error message.

## What You've Learnt

- **CSRF protection** — the `csrf` app provides middleware and a `csrfToken` template function
- **Flash messages** — `messages.AddSuccess()` / `AddError()` store messages in the session, displayed on the next page load
- **Redirect-after-POST** — prevents duplicate submissions by redirecting with 303

## Next

In [Part 5](part5.md), you'll add authentication so that only logged-in users can vote.
