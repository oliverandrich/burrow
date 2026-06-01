# Auth

WebAuthn (passkey) authentication with recovery codes, email verification, and invite-only registration.

**Package:** `github.com/oliverandrich/burrow/contrib/auth`

**Depends on:** `session`, `csrf`, `staticfiles` (hard — server fails at boot if any is missing). Soft-integrates with `jobs` (queued email delivery; falls back to synchronous send) and ships English + German translations via `HasTranslations` for the framework's always-on i18n bundle.

!!! note "Generic over a Profile type"
    `auth.User`, `auth.App`, `auth.Repository`, `auth.CurrentUser`, and `auth.MustCurrentUser` are all generic over a `Profile` type parameter that holds app-specific extension fields. Examples below use `auth.EmptyProfile` for apps that don't extend the user. To add display name, bio, avatar, social links, or other custom fields, see [Extending the User](auth-profile.md).

!!! info "JavaScript required"
    WebAuthn calls `navigator.credentials.create()` / `.get()`, so registration and sign-in cannot complete without JavaScript. Every page protected by `RequireAuth` therefore sits behind a JS-capable client; auth-rendered forms (recovery-code acknowledgement, admin user edit, logout) use `hx-post` and do not carry a `method=post` fallback. The public auth layout (`auth/layout`) renders a `<noscript>` block at the top of every login/register/recovery page so a JS-off browser sees a translated explanation rather than a silently-broken passkey button. See [admin → JavaScript required](admin.md#javascript-required) for the full convention.

## Setup

```go
srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    auth.New[auth.EmptyProfile](),
    htmx.New(),
    admin.New(),
    staticApp, // staticfiles.New(emptyFS) — returns (*App, error)
    // ... other apps
)
```

The constructor uses a built-in auth layout. Override it with an option, and restyle individual pages by redefining their templates (see [Page rendering](#page-rendering)):

```go
// With a custom auth layout shell.
auth.New[auth.EmptyProfile](
    auth.WithAuthLayout("myapp/auth-layout"),
)
```

Other constructor options:

- `auth.WithEmailService(svc)` — wire up an email backend (see [Email Service](#email-service)).
- `auth.WithLogoComponent(html)` — render a small logo above login/register/recovery pages.
- `auth.WithDefaultRole(role)` — assign every newly-registered user a role other than `RoleUser` (see [Default Role for New Users](#default-role-for-new-users)).
- `auth.WithUsernameValidator(fn)` / `auth.WithEmailValidator(fn)` — reject chosen usernames or email addresses at registration time (see [Registration Validators](#registration-validators)).

## Registration Validators

Uniqueness of usernames and emails is already enforced by the database. The validators are for *policy* rejections — reserved handles, blocked addresses, domain restrictions — values that are free but unwanted.

```go
reserved := map[string]bool{"posts": true, "notes": true, "all": true}

srv.Register(auth.New[auth.EmptyProfile](
    auth.WithUsernameValidator[auth.EmptyProfile](func(ctx context.Context, username string) error {
        if reserved[username] {
            return fmt.Errorf("username %q is reserved", username)
        }
        return nil
    }),
))
```

The matching validator runs during registration after the value is read and any invite is checked, but before the user is created. `WithUsernameValidator` runs only in username mode; `WithEmailValidator` only in email mode — the modes are mutually exclusive, so exactly one fires. A non-nil error aborts registration with HTTP 400.

!!! warning "The error message is shown to the user"
    The validator's error message is returned verbatim in the registration response and surfaced by the WebAuthn client. Return a clean, user-facing message (e.g. `"username is reserved"`), not a wrapped internal error.

## Default Templates

The auth app ships HTML templates via `HasTemplates`. These templates use the global template set and are rendered with `burrow.Render()`. The auth app also implements `HasRequestFuncMap` to provide `currentUser`, `isAuthenticated`, and other request-scoped functions available in all templates.

**Note:** When using default templates, register the `staticfiles` app so that `webauthn.js` is served. The auth app implements `HasStaticFiles` and contributes its assets under the `"auth"` prefix automatically.

## Auth Layout

Public auth pages (login, register, recovery, email verification) use a separate layout — a minimal page without the full app navigation. This avoids showing the navbar to unauthenticated users. Authenticated routes (`/auth/credentials`, `/auth/recovery-codes`) continue to use the global app layout.

By default, `auth.New[auth.EmptyProfile]()` ships its own `auth/layout` template: a Tailwind-styled, navbar-less shell that links to `{{ staticURL "app/app.min.css" }}` for the host app's compiled stylesheet. Registering `auth.New[auth.EmptyProfile]()` works out of the box — no `WithAuthLayout` call required — as long as your host serves its CSS at that static path (the convention the [Tailwind guide](../guide/tailwind.md) describes for an in-tree app shell).

Hosts that serve CSS at a different path or want a different shell override the layout with `auth.WithAuthLayout("myapp/auth-layout")`:

```go
auth.New[auth.EmptyProfile](
    auth.WithAuthLayout("myapp/auth-layout"),
)
```

Passing the empty string is also accepted — auth pages then inherit whatever the host set via `srv.SetLayout`. That was the pre-v0.20 behaviour and is generally not what you want (the host's navbar bleeds into login pages).

An auth layout is simply a template name string referring to a template in the global template set:

```html
{{ define "myapp/auth-layout" -}}
<!DOCTYPE html>
<html lang="{{ lang }}">
<head>
    <meta charset="utf-8">
    <title>{{ .Title }}</title>
    <link rel="stylesheet" href="{{ staticURL "app/app.min.css" }}">
</head>
<body class="min-h-screen bg-white text-gray-900 dark:bg-gray-900 dark:text-gray-100">
    <main class="mx-auto max-w-md p-6">
        {{ .Content }}
    </main>
</body>
</html>
{{- end }}
```

See [Layouts & Rendering](../guide/layouts.md) for more details on how layouts work.

## Models

### User

`auth.User[P]` carries the auth-core fields. Apps that need display name, bio, avatar, or other extension fields put them on the `Profile P` type — see [Extending the User](auth-profile.md).

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `string` | ULID, generated on insert |
| `Username` | `string` | Unique username |
| `Email` | `*string` | Optional, unique |
| `Role` | `string` | One of `RoleUser` / `RoleStaff` / `RoleAdmin` (string values `"user"`, `"staff"`, `"admin"`) |
| `IsActive` | `bool` | Whether the user can log in |
| `EmailVerified` | `bool` | Whether email is verified |
| `EmailVerifiedAt` | `*time.Time` | When email was verified |
| `Profile` | `P` | App-defined extension struct (see [Extending the User](auth-profile.md)) |
| `Credentials` | `[]Credential` | WebAuthn credentials (populated by separate query) |

#### Role predicates

Roles form a monotonic hierarchy: `admin` implies `staff` implies `authenticated`. Two helpers encode that — always prefer them over direct `u.Role == "..."` comparisons so calling code keeps working if the role set is extended later.

| Method | Returns true when |
|--------|-------------------|
| `User.IsStaff()` | `Role` is `RoleStaff` or `RoleAdmin` |
| `User.IsAdmin()` | `Role` is `RoleAdmin` |

The framework reads the same predicates via `burrow.IsStaff(ctx)` / `burrow.IsAdmin(ctx)` (set up by the auth middleware via `burrow.AuthChecker`) so template-level filtering (`NavItem.StaffOnly`, `AdminOnly`) and middleware (`RequireStaff`, `RequireAdmin`) stay consistent with what `u.IsStaff()` reports.

### Credential

Stores a WebAuthn public key credential bound to a user.

### RecoveryCode

Bcrypt-hashed one-time recovery codes for account access when passkeys are unavailable.

### Invite

Time-limited invite tokens for invite-only registration.

### APIKey

SHA256-hashed API key (personal access token) for bearer authentication of non-browser clients (fields: `ID`, `UserID`, `Label`, optional `ExpiresAt`). Only the hash is stored — the plaintext is shown once at creation. A key inherits the role of its owning user. See [API Key Authentication](#api-key-authentication).

## Routes

All routes are registered under `/auth`:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/auth/register` | Registration page |
| POST | `/auth/register/begin` | Start WebAuthn registration |
| POST | `/auth/register/finish` | Complete WebAuthn registration |
| GET | `/auth/login` | Login page |
| POST | `/auth/login/begin` | Start WebAuthn login |
| POST | `/auth/login/finish` | Complete WebAuthn login |
| POST | `/auth/logout` | Log out |
| GET | `/auth/recovery` | Recovery code login page |
| POST | `/auth/recovery` | Log in with recovery code |
| GET | `/auth/credentials` | Manage credentials (auth required) |
| POST | `/auth/credentials/begin` | Add credential (auth required) |
| POST | `/auth/credentials/finish` | Complete add credential (auth required) |
| DELETE | `/auth/credentials/:id` | Delete credential (auth required) |
| GET | `/auth/recovery-codes` | View recovery codes (auth required) |
| POST | `/auth/recovery-codes/ack` | Acknowledge recovery codes (auth required) |
| POST | `/auth/recovery-codes/regenerate` | Regenerate recovery codes (auth required) |
| GET | `/auth/verify-pending` | Email verification pending page |
| GET | `/auth/verify-email` | Verify email via token |
| POST | `/auth/resend-verification` | Resend verification email |

When `POST /auth/recovery` consumes the user's final unused recovery code, the handler auto-regenerates a fresh set and redirects to `/auth/recovery-codes` (the same ack flow as `POST /auth/recovery-codes/regenerate`), so a passkey-only account never reaches a "logged in with zero codes left" state.

Admin routes (registered via `HasAdmin`). The `/admin/` frame is gated by `RequireAuth + RequireStaff`; `contrib/auth` self-gates every route below with `RequireAdmin()` because user and invite management is admin-only:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/admin/users` | List users |
| GET | `/admin/users/{id}` | User detail / edit form |
| POST | `/admin/users/{id}` | Update user |
| DELETE | `/admin/users/{id}` | Delete user |
| POST | `/admin/users/{id}/deactivate` | Deactivate user |
| POST | `/admin/users/{id}/activate` | Activate user |
| GET | `/admin/invites` | List invites |
| GET | `/admin/invites/new` | New invite form (htmx fragment) |
| POST | `/admin/invites` | Create an invite |
| DELETE | `/admin/invites/{id}/revoke` | Revoke an invite |

## Template Functions

The auth app contributes these template functions:

**Via `HasRequestFuncMap` (request-scoped):**

| Function | Example | Description |
|----------|---------|-------------|
| `currentUser` | `{{ if $u := currentUser }}{{ $u.Profile.Name }}{{ end }}` | Returns the authenticated `*User[P]` (with `Profile` populated), or `nil` |
| `isAuthenticated` | `{{ if isAuthenticated }}Sign out{{ else }}Sign in{{ end }}` | Returns `true` if a user is logged in |

These are available in all templates and are commonly used in layout navigation. `currentUser` returns the typed user — when `auth.New[myapp.Profile]()` is registered, `currentUser` returns `*User[myapp.Profile]` and Profile fields are reachable directly. Recommended display-name pattern (fall back to `Username` when the Profile field is empty):

```html
<span>{{ if $u := currentUser }}{{ if $u.Profile.Name }}{{ $u.Profile.Name }}{{ else }}{{ $u.Username }}{{ end }}{{ end }}</span>
```

See [Extending the User](auth-profile.md) for the full Profile story.

## Middleware

The auth app provides three middleware functions:

### Automatic User Loading

Registered automatically — resolves the user on every request from the session cookie or, for non-browser clients, an `Authorization: Bearer <api-key>` header (see [API Key Authentication](#api-key-authentication)). Both populate the same context:

```go
// In any handler, after auth middleware runs:
user := auth.CurrentUser[auth.EmptyProfile](r.Context())  // or nil if unauthenticated
```

### RequireAuth

Denies unauthenticated requests. Browsers and htmx requests are redirected to the login page (the original target is stored in the session for post-login return); API clients (`Accept: application/json`) get a **401** with a `WWW-Authenticate: Bearer` challenge instead.

```go
r.Route("/notes", func(r chi.Router) {
    r.Use(auth.RequireAuth())
    // ... routes
})
```

### RequireStaff

Denies unauthenticated requests (redirect for browsers, 401 for API clients); returns 403 if the user is neither `staff` nor `admin`. The `contrib/admin` coordinator uses this to gate the `/admin/` frame, but you can apply it to your own routes too (e.g. a `/studio/` shell):

```go
r.Route("/studio", func(r chi.Router) {
    r.Use(auth.RequireStaff())
    // ... routes
})
```

### RequireAdmin

Denies unauthenticated requests (redirect for browsers, 401 for API clients); returns 403 if the user is not an admin. Use this inside `HasAdmin.AdminRoutes` to gate admin-only sub-groups (the `/admin/` frame itself is staff-gated; admin-only routes self-gate):

```go
func (a *App) AdminRoutes(r chi.Router) {
    r.Group(func(r chi.Router) {
        r.Use(auth.RequireAdmin())
        r.Get("/settings", a.settings)
    })
}
```

### API Key Authentication

Non-browser clients authenticate with an API key (personal access token) sent as `Authorization: Bearer <token>`. There is **no separate middleware** — the automatic user-loading middleware resolves a bearer token to its owning user just like a session cookie, so the standard gates work unchanged. Gate an API the same way you gate any route:

```go
r.Route("/api", func(r chi.Router) {
    r.Use(auth.RequireAuth())   // or RequireStaff() — keys inherit their owner's role
    // ... resource routes
})
```

A valid key authenticates as its owner. A missing, unknown, or expired key (or one whose user is inactive) leaves the request unauthenticated, so `RequireAuth` answers API clients with a 401 (see above). Send `Accept: application/json` so failures return that 401 rather than a login redirect:

```bash
curl -H "Authorization: Bearer brw_…" -H "Accept: application/json" https://example.com/api/notes
```

!!! warning "Exempt API routes from CSRF"
    The bearer path carries no cookie, so it needs no CSRF token — but the global `csrf` middleware still rejects unsafe methods (POST/PUT/DELETE) that arrive without one. Declare your API prefix CSRF-exempt via the [`csrf.ExemptPaths`](csrf.md#exempting-webhook-paths) capability interface on the app that owns the routes. The trailing slash makes it a **prefix** match (covers `/api/notes`, `/api/notes/123`, …):

    ```go
    func (a *App) CSRFExemptPaths() []string { return []string{"/api/"} }
    ```

#### Managing keys

There is no built-in UI or CLI command yet — you mint and revoke keys yourself through the auth repository (the same `*Repository[P]` your app already builds in `Configure`). The plaintext token is returned **once** at creation; show it to the user immediately and never persist it (only the hash is stored):

```go
func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
    a.userRepo = auth.NewRepository[Profile](cfg.DB) // store on the app
    return nil
}

// In an authenticated handler — e.g. a "create token" action on a settings page:
func (a *App) createKey(w http.ResponseWriter, r *http.Request) error {
    user := auth.MustCurrentUser[Profile](r.Context())

    // nil expiry = never expires; pass a *time.Time to set one.
    plaintext, _, err := a.userRepo.CreateAPIKey(r.Context(), user.ID, "ci-deploy", nil)
    if err != nil {
        return err
    }
    // Show `plaintext` (e.g. "brw_…") to the user NOW — it is unrecoverable afterwards.
    return burrow.JSON(w, http.StatusCreated, map[string]string{"token": plaintext})
}

// Listing and revoking a user's keys (hashes only; plaintext is gone):
keys, _ := a.userRepo.ListAPIKeysByUser(ctx, user.ID)
_ = a.userRepo.DeleteAPIKey(ctx, keyID, user.ID) // revoke, scoped to the owner
```

## Context Helpers

In Go code:

```go
user := auth.CurrentUser[auth.EmptyProfile](r.Context())    // or nil if unauthenticated
if auth.IsAuthenticated(r.Context()) { ... }
```

!!! warning "Match the Profile type parameter"
    The type parameter on `CurrentUser` / `MustCurrentUser` **must match** the one passed to `auth.New[P]()`. If you registered `auth.New[myapp.Profile]()` but read with `auth.CurrentUser[auth.EmptyProfile]`, the type assertion misses silently and returns `nil` — no panic, no log. `IsAuthenticated` still reports `true` (it only checks for context-value presence). Stick to one Profile type throughout the app and substitute it wherever you see `[auth.EmptyProfile]` in the examples below.

### MustCurrentUser

`MustCurrentUser` returns the authenticated user from the context or panics if no user is present. Use it **only** in handlers that are protected by `RequireAuth` middleware, where the nil case is unreachable.

This eliminates the repetitive nil-check boilerplate:

**Before:**

```go
func (a *App) List(w http.ResponseWriter, r *http.Request) error {
    user := auth.CurrentUser[auth.EmptyProfile](r.Context())
    if user == nil {
        return burrow.NewHTTPError(http.StatusUnauthorized, "not authenticated")
    }
    // use user ...
}
```

**After:**

```go
func (a *App) List(w http.ResponseWriter, r *http.Request) error {
    user := auth.MustCurrentUser[auth.EmptyProfile](r.Context())
    // use user ...
}
```

!!! warning
    `MustCurrentUser` panics with a descriptive message if called without an authenticated user in the context. Only use it behind `RequireAuth` middleware. For handlers that may be accessed without authentication, continue using `CurrentUser` with a nil check.

In templates (via `HasRequestFuncMap`):

```html
{{ if isAuthenticated }}
    <span>Hello, {{ (currentUser).Username }}</span>
{{ end }}
```

## Page rendering

The auth app renders its pages (login, register, credentials, recovery codes, email verification) with the shipped `auth/*` templates — Tailwind v4 markup wrapped in either a centered layout (login) or a card layout (the rest). `auth.DefaultAuthLayout()` returns `"auth/layout"`, the navbar-less shell from [Auth Layout](#auth-layout), so auth pages render inside it by default. Pass `auth.WithAuthLayout("")` to inherit the host's `srv.SetLayout` instead, or `auth.WithAuthLayout("myapp/auth-layout")` to swap in your own shell.

To restyle a page, **redefine its template**. Ship the block in your own app's templates (contributed via `HasTemplates` — see [Templates](#default-templates)); burrow parses every app's templates into one set and last definition wins, so an app registered after `auth` overrides the built-in block:

```go
{{ define "auth/login" }}
  <!-- your markup; the page's data is unchanged (see the table below) -->
{{ end }}
```

Overrides are markup-only: the data each page receives is fixed (there is no longer a renderer to pass custom Go-side data). The page templates and their data:

| Template | Data available |
|----------|----------------|
| `auth/login` | `.LoginRedirect` |
| `auth/register` | `.UseEmail`, `.InviteOnly`, `.Email`, `.Invite` |
| `auth/credentials` | `.Creds` (`[]Credential`) |
| `auth/recovery` | `.LoginRedirect` |
| `auth/recovery_codes` | `.Codes` (`[]string`) |
| `auth/verify_pending` / `auth/verify_success` | — |
| `auth/verify_error` | `.ErrorCode` |

The `auth/layout`, `auth/card`, and `auth/centered` wrappers receive `.Content`, `.Title`, and (card only) `.CardTitle`. No renderer abstraction to implement — template redefinition is the override mechanism.

## Admin Integration

The auth app implements `HasAdmin` to provide user and invite management in the admin panel with hand-written handlers. Admin views include user list with search and role filter, user edit form with last-admin demotion protection, and an htmx-powered inline invite creation form.

## Default Role for New Users

By default, newly-registered users land in `RoleUser`. For apps where every accepted invitee is expected to be staff — for example invite-only blog engines or internal tools with no reader role — `auth.WithDefaultRole` skips the "register, then promote" round-trip:

```go
auth.New[auth.EmptyProfile](
    auth.WithDefaultRole(auth.RoleStaff),
)
```

The first-user-becomes-admin promotion still wins: the very first user in an empty database becomes `RoleAdmin` regardless of this option, so a fresh deployment never locks itself out of `/admin/`. After that, every successful registration is assigned the configured role.

Valid arguments are `auth.RoleUser`, `auth.RoleStaff`, `auth.RoleAdmin`, or the empty string (which behaves identically to not passing the option). Any other value makes `Configure` return an error at boot.

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--auth-login-redirect` | `AUTH_LOGIN_REDIRECT` | `/` | Redirect after login |
| `--auth-logout-redirect` | `AUTH_LOGOUT_REDIRECT` | `/auth/login` | Redirect after logout |
| `--auth-use-email` | `AUTH_USE_EMAIL` | `false` | Use email instead of username |
| `--auth-require-verification` | `AUTH_REQUIRE_VERIFICATION` | `false` | Require email verification |
| `--auth-invite-only` | `AUTH_INVITE_ONLY` | `false` | Require invite to register |
| `--auth-webauthn-rp-id` | `WEBAUTHN_RP_ID` | (URL host) | WebAuthn Relying Party ID; falls back to the host of `--base-url` |
| `--auth-webauthn-rp-display-name` | `WEBAUTHN_RP_DISPLAY_NAME` | (`--app-name`) | Display name shown in the browser passkey dialog. Falls back to the global `--app-name` (binary basename by default), so most apps don't set this explicitly. |
| `--auth-webauthn-rp-origin` | `WEBAUTHN_RP_ORIGIN` | (base URL) | WebAuthn RP origin |

## Email Service

For email verification and invite emails, wire up the [authmail](authmail.md) SMTP app or implement the `EmailService` interface:

```go
type EmailService interface {
    SendVerification(ctx context.Context, toEmail, verifyURL string) error
    SendInvite(ctx context.Context, toEmail, inviteURL string) error
}
```

### Job-based delivery (recommended)

When a `burrow.Queue` implementation is registered (e.g., [`contrib/jobs`](jobs.md)), the auth app automatically delivers emails via the job queue. This gives you:

- **Automatic retries** with exponential backoff (5 retries by default)
- **Persistence** — emails survive server restarts
- **Admin visibility** — failed deliveries are visible in the jobs admin UI

```go
srv := burrow.NewServer(
    session.New(),
    jobs.New(),   // register a queue — auth will use it automatically
    auth.New[auth.EmptyProfile](
        auth.WithEmailService(mailer),
    ),
    // ...
)
```

The auth app implements `burrow.HasJobs` and registers an `auth.send_email` job handler. The queue implementation discovers it during `Configure()` — no manual wiring needed.

Without a queue, emails are sent directly (synchronously in the request handler). This works for development but is not recommended for production, since transient SMTP failures will cause the email to be lost.

## Internationalisation

The auth app implements `HasTranslations` and ships English and German translations for all user-facing strings. When the `i18n` contrib app is registered, translations are auto-discovered and loaded.

**Email i18n:** Auth emails (verification, invite) are sent in the user's locale. The locale is captured from the request context and serialized into the job payload. The job handler restores the locale via `i18n.App.WithLocale()` before rendering the email. The default email renderer (`auth.DefaultEmailRenderer()`) uses `i18n.T()` for all translatable strings.

Without the i18n app, templates fall back to displaying translation keys (which match their English text). To add a custom language, create a TOML file (e.g., `active.fr.toml`) with the same keys as `active.en.toml` and contribute it via the `HasTranslations` interface in your app.

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()` |
| `HasDocuments` | User, credential, recovery code, invite document types |
| `HasRoutes` | Auth routes |
| `HasMiddleware` | User loading from session |
| `HasAdmin` | Admin user/invite management routes and nav items |
| `HasStaticFiles` | Contributes embedded `webauthn.js` under `"auth"` prefix |
| `HasTemplates` | Contributes auth HTML templates |
| `HasRequestFuncMap` | Provides `currentUser`, `isAuthenticated` to templates |
| `HasTranslations` | Contributes English and German translation files |
| `HasFuncMap` | Provides `credName`, `deref` to templates |
| `Configurable` | Auth and WebAuthn flags |
| `HasDependencies` | Requires `session` |
| `HasJobs` | Registers `auth.send_email` job handler for email delivery via queue |
| `HasCLICommands` | Provides `set-role`, `create-invite` subcommands |
| `HasShutdown` | Stops the background WebAuthn challenge cleanup goroutine |
