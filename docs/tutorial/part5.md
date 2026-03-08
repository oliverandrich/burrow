# Part 5: Authentication

In this part you'll add the auth contrib app so that only logged-in users can vote.

**Source code:** [`tutorial/step05/`](https://codeberg.org/oliverandrich/burrow/src/branch/main/tutorial/step05)

## Add the Auth App

The `auth` contrib app provides WebAuthn (passkey) authentication with registration, login, and logout flows. It ships with default templates for all auth pages.

Update `main.go`:

```go
import (
    "codeberg.org/oliverandrich/burrow/contrib/auth"
    "codeberg.org/oliverandrich/burrow/contrib/healthcheck"
)

srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    staticApp,
    healthcheck.New(),
    messages.New(),
    bootstrap.New(),
    pages.New(),
    auth.New(),           // new
    polls.New(),
)
```

The auth app:

- Provides routes at `/auth/login`, `/auth/register`, `/auth/logout`
- Runs its own database migrations for user and credential tables
- Injects the current user into request context via middleware

## Protect the Vote Route

Use `auth.RequireAuth()` middleware to restrict voting to authenticated users:

```go
func (a *App) Routes(r chi.Router) {
    r.Route("/polls", func(r chi.Router) {
        r.Get("/", burrow.Handle(a.handlers.List))
        r.Get("/{id}", burrow.Handle(a.handlers.Detail))
        r.Get("/{id}/results", burrow.Handle(a.handlers.Results))

        // Voting requires authentication.
        r.Group(func(r chi.Router) {
            r.Use(auth.RequireAuth())
            r.Post("/{id}/vote", burrow.Handle(a.handlers.Vote))
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

Update the layout to display the current user's email and a sign-out button:

```go
layoutData := map[string]any{
    "Content":  content,
    "NavItems": burrow.NavItems(r.Context()),
    "Messages": messages.Get(r.Context()),
    "User":     auth.UserFromContext(r.Context()),  // new
}
```

In the layout template:

```html
<ul class="navbar-nav">
    {{ if .User -}}
    <li class="nav-item">
        <span class="nav-link text-body-secondary">{{ .User.Email }}</span>
    </li>
    <li class="nav-item">
        <form method="post" action="/auth/logout">
            <input type="hidden" name="gorilla.csrf.Token" value="{{ csrfToken }}">
            <button type="submit" class="btn btn-link nav-link">Sign out</button>
        </form>
    </li>
    {{ else -}}
    <li class="nav-item">
        <a class="nav-link" href="/auth/login">Sign in</a>
    </li>
    {{ end -}}
</ul>
```

## Run It

```bash
go run .
```

Visit `/auth/register` to create an account (you'll need a browser that supports passkeys/WebAuthn). After registering, try voting — it should work. Sign out and try again — you'll be redirected to the login page.

## What You've Learnt

- **`auth.New()`** — configures the auth app with built-in default renderer and layout
- **`auth.RequireAuth()`** — middleware that redirects unauthenticated users to login
- **`auth.UserFromContext()`** — retrieves the authenticated user from request context
- **`HasDependencies`** — declares inter-app dependencies for automatic ordering

## Next

In [Part 6](part6.md), you'll add an admin panel to manage questions without touching the database directly.
