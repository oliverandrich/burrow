# Part 4: Forms, CRUD & Validation

In this part you'll add a voting form with CSRF protection, flash messages, and the redirect-after-POST pattern.

**Source code:** [`tutorial/step04/`](https://github.com/oliverandrich/burrow/tree/main/tutorial/step04)

## New Contrib Apps

This step introduces two new contrib apps:

- **`csrf`** — CSRF protection via gorilla/csrf. Injects a `csrfToken` template function.
- **`messages`** — Flash messages that survive redirects. Stored in the session.

Update `main.go` — add the new imports and apps:

```go
import (
    "github.com/oliverandrich/burrow/contrib/csrf"
    "github.com/oliverandrich/burrow/contrib/messages"
    "github.com/oliverandrich/burrow/contrib/session"
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
    app.New(),
    polls.New(),
)
```

## Add a Voting Form

Update `internal/polls/templates/polls/detail.html` to include a form with radio buttons:

```html
{{ define "polls/detail" -}}
<header>
    <h1>{{ .Question.Text }}</h1>
</header>
<form method="post" action="/polls/{{ .Question.ID }}/vote">
    {{ csrfField }}
    <fieldset class="poll-choices">
        {{ range .Question.Choices -}}
        <label>
            <input type="radio" name="choice" value="{{ .ID }}">
            {{ .Text }}
        </label>
        {{ end -}}
    </fieldset>
    <div role="group">
        <button type="submit" class="btn btn-primary">Vote</button>
        <a href="/polls" role="button" class="btn btn-outline btn-secondary">&laquo; Back to polls</a>
    </div>
</form>
<style>
.poll-choices{border:none;padding:0;margin-bottom:1rem}
.poll-choices label{display:flex;align-items:center;gap:.5rem;padding:.5rem 0}
</style>
{{- end }}
```

Key points:

- **`{{ csrfField }}`** is a template function provided by the `csrf` app via `HasRequestFuncMap`. It renders a hidden input field containing the CSRF token for the current request.
- Without a valid token, the POST request will be rejected with a 403.

!!! tip "For complex forms, use the forms package"
    This tutorial uses `r.FormValue()` directly because the voting form has a single radio-button field — no model binding or per-field error rendering needed. For forms with multiple fields, validation, and error display, see the [Forms guide](../guide/forms.md).

## Handle the Vote

All the following changes go into `internal/polls/polls.go`. First, add the `messages` import:

```go
"github.com/oliverandrich/burrow/contrib/messages"
```

Add the `IncrementVotes` method to the repository:

```go
func (r *Repository) IncrementVotes(ctx context.Context, choiceID string) error {
    choice, err := den.FindByID[Choice](ctx, r.db, choiceID)
    if err != nil {
        return err
    }
    choice.Votes++
    return den.Replace(ctx, r.db, choice)
}
```

Then add a `Vote` handler method on `*App`:

```go
func (a *App) Vote(w http.ResponseWriter, r *http.Request) error {
    questionID := chi.URLParam(r, "id")
    if questionID == "" {
        return burrow.NewHTTPError(http.StatusBadRequest, "invalid question ID")
    }

    choiceID := r.FormValue("choice")
    if choiceID == "" {
        if addErr := messages.AddError(w, r, "You didn't select a choice."); addErr != nil {
            return addErr
        }
        http.Redirect(w, r, fmt.Sprintf("/polls/%s", questionID), http.StatusSeeOther)
        return nil
    }

    if err := a.repo.IncrementVotes(r.Context(), choiceID); err != nil {
        return burrow.NewHTTPError(http.StatusInternalServerError, "failed to record vote")
    }

    if err := messages.AddSuccess(w, r, "Your vote has been recorded!"); err != nil {
        return err
    }
    http.Redirect(w, r, fmt.Sprintf("/polls/%s/results", questionID), http.StatusSeeOther)
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
        r.Get("/", burrow.Handle(a.List))
        r.Get("/{id}", burrow.Handle(a.Detail))
        r.Post("/{id}/vote", burrow.Handle(a.Vote))  // new
        r.Get("/{id}/results", burrow.Handle(a.Results))
    })
}
```

## Display Flash Messages

Update the layout template in `internal/app/templates/app/layout.html` to show messages above the content:

```html
<main class="container">
    {{ range messages -}}
    <div class="alert alert-{{ .Level }}" role="alert">{{ .Text }}</div>
    {{ end -}}
    {{ .Content }}
</main>
```

`messages` is a template function provided by the `messages` contrib app via `HasRequestFuncMap` — it returns the flash messages for the current request without you having to plumb them through the template data manually.

Each `Message` has a `Level` (success, error, warning, info) and `Text`. The level maps to one of the alert classes shipped in `app.css` — `.alert-success`, `.alert-error`, `.alert-warning`, `.alert-info`.

## Run It

```bash
go mod tidy
go run .
```

Seed some test data, then navigate to a question. Select a choice and click "Vote" — you'll be redirected to the results page with a success message. Try submitting without selecting a choice to see the error message.

## What You've Learnt

- **CSRF protection** — the `csrf` app provides middleware and a `csrfToken` template function
- **Flash messages** — `messages.AddSuccess()` / `AddError()` store messages in the session, displayed on the next page load
- **Redirect-after-POST** — prevents duplicate submissions by redirecting with 303

## Next

In [Part 5](part5.md), you'll add authentication so that only logged-in users can vote.
