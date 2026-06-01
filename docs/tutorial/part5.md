# Part 5: Authentication

In this part you'll add the auth contrib app so that only logged-in users can vote.

**Source code:** [`tutorial/step05/`](https://github.com/oliverandrich/burrow/tree/main/tutorial/step05)

## Add the Auth App

The `auth` contrib app provides WebAuthn (passkey) authentication with registration, login, and logout flows. It ships with default templates for all auth pages.

Update `main.go`:

```go
import (
    "github.com/oliverandrich/burrow/contrib/auth"
)

srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    staticApp,
    healthcheck.New(),
    messages.New(),
    app.New(),
    auth.New[auth.EmptyProfile](),           // new
    polls.New(),
)
```

The auth app:

- Provides routes at `/auth/login`, `/auth/register`, `/auth/logout`
- Runs its own database migrations for user and credential tables
- Injects the current user into request context via middleware

## Protect the Vote Route

In `internal/polls/polls.go`, add the `auth` import:

```go
"github.com/oliverandrich/burrow/contrib/auth"
```

Then update the `Routes` method to use `auth.RequireAuth()` middleware, restricting voting to authenticated users:

```go
func (a *App) Routes(r chi.Router) {
    r.Route("/polls", func(r chi.Router) {
        r.Get("/", burrow.Handle(a.List))
        r.Get("/{id}", burrow.Handle(a.Detail))
        r.Get("/{id}/results", burrow.Handle(a.Results))

        // Voting requires authentication.
        r.Group(func(r chi.Router) {
            r.Use(auth.RequireAuth())
            r.Post("/{id}/vote", burrow.Handle(a.Vote))
        })
    })
}
```

`chi.Router.Group()` creates a sub-router with its own middleware stack. Only the vote route requires login — browsing questions and viewing results remain public.

If an unauthenticated user tries to vote, they'll be redirected to `/auth/login`. After logging in, they'll return to the page they came from.

## Declare the Dependency

Since the polls app now depends on the auth app, declare it:

```go
func (a *App) Dependencies() []string { return []string{"auth"} }
```

Burrow automatically sorts apps by dependencies during `NewServer()`, so you don't need to worry about registration order.

## Show the User in the Navbar

No Go change is needed here: the auth contrib already exposes `currentUser` and `isAuthenticated` to every template via `HasRequestFuncMap`, and the framework injects flash messages via `messages` and the locale via `lang` the same way. The layout reads those functions directly without any data plumbing through `Render`.

Update the navbar in `internal/app/templates/app/layout.html`. Add a `<li class="spacer"></li>` between the nav items and the user controls — the `.spacer` rule in `app.css` is a `flex: 1` filler that pushes the username and sign-out button to the right edge:

```html
<nav class="topnav">
    <ul>
        <li><a href="/" class="brand">Polls</a></li>
        {{ range navLinks -}}
        <li><a href="{{ .URL }}"{{ if .IsActive }} aria-current="page"{{ end }}>{{ .Label }}</a></li>
        {{ end -}}
        <li class="spacer"></li>
        {{ if currentUser -}}
        <li>{{ (currentUser).Username }}</li>
        <li>
            <form method="post" action="/auth/logout">
                {{ csrfField }}
                <button type="submit" class="btn btn-outline btn-sm">Sign out</button>
            </form>
        </li>
        {{ else -}}
        <li><a href="/auth/login">Sign in</a></li>
        {{ end -}}
    </ul>
</nav>
```

Logout has to be a POST to defend against CSRF, so we use a small inline `<form>` with the submit button styled as an outline button via the `.btn .btn-outline .btn-sm` classes from `app.css`.

`currentUser` is a template function provided by the auth app via `HasRequestFuncMap` — it returns the logged-in user (or nil) for the current request, so you don't need to plumb it through the layout data manually.

The parentheses around `currentUser` are required because it's a function — `currentUser.Username` would try to look up a `Username` field on the function value itself. `(currentUser).Username` calls the function first, then accesses the field on the returned `*auth.User[auth.EmptyProfile]`.

We'll polish this navbar into a proper dropdown menu in [Part 7](part7.md) once we have htmx loaded — the inline `<form>` is a clean fallback that works without any client-side JavaScript.

## Run It

```bash
go mod tidy
go run .
```

Visit `/auth/register` to create an account (you'll need a browser that supports passkeys/WebAuthn). After registering, try voting — it should work. Sign out and try again — you'll be redirected to the login page.

## What You've Learnt

- **`auth.New[auth.EmptyProfile]()`** — configures the auth app with the built-in auth layout and page templates
- **`auth.RequireAuth()`** — middleware that redirects unauthenticated users to login
- **`auth.CurrentUser[auth.EmptyProfile]()`** — retrieves the authenticated user from request context
- **`HasDependencies`** — declares inter-app dependencies for automatic ordering

## Next

In [Part 6](part6.md), you'll add an admin panel to manage questions without touching the database directly.
