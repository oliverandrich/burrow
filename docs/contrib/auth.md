# Auth

WebAuthn (passkey) authentication with recovery codes, email verification, and invite-only registration.

**Package:** `codeberg.org/oliverandrich/burrow/contrib/auth`

**Depends on:** `session`

## Setup

```go
srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    auth.New(),
    bootstrap.New(),
    htmx.New(),
    admin.New(),
    staticfiles.New(emptyFS), // serves auth + admin static files
    // ... other apps
)
```

`auth.New()` uses built-in defaults for the renderer, auth layout, and admin renderer. Use options to override with custom implementations:

```go
// With custom renderer and layout.
auth.New(
    auth.WithRenderer(myCustomRenderer),
    auth.WithAuthLayout(myCustomAuthLayout),
    auth.WithAdminRenderer(myCustomAdminRenderer),
)
```

## Default Templates

The auth app ships HTML templates via `HasTemplates`. These templates use the global template set and are rendered with `burrow.RenderTemplate()`. The auth app also implements `HasRequestFuncMap` to provide `currentUser`, `isAuthenticated`, and other request-scoped functions available in all templates.

**Note:** When using default templates, register the `staticfiles` app so that `webauthn.js` is served. The auth app implements `HasStaticFiles` and contributes its assets under the `"auth"` prefix automatically.

## Auth Layout

By default, `auth.New()` uses a built-in minimal auth layout (`DefaultAuthLayout()`) for public auth pages (login, register, recovery, email verification). This avoids showing the full app navbar to unauthenticated users. Authenticated routes (`/auth/credentials`, `/auth/recovery-codes`) continue to use the global app layout.

Use `auth.WithAuthLayout()` to override the default auth layout with a custom one:

```go
auth.New(
    auth.WithAuthLayout(myCustomAuthLayout),
)
```

This follows the same pattern as the admin app, which overrides the layout inside its `/admin` route group.

## Models

### User

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `int64` | Primary key |
| `Username` | `string` | Unique username |
| `Role` | `string` | `"user"` or `"admin"` |
| `Email` | `*string` | Optional, unique |
| `EmailVerified` | `bool` | Whether email is verified |
| `Name` | `string` | Display name |
| `Bio` | `string` | User bio |
| `Credentials` | `[]Credential` | WebAuthn credentials (eager loaded) |

### Credential

Stores a WebAuthn public key credential bound to a user.

### RecoveryCode

Bcrypt-hashed one-time recovery codes for account access when passkeys are unavailable.

### Invite

Time-limited invite tokens for invite-only registration.

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

Admin routes (registered via `HasAdmin`, require auth + admin role):

| Method | Path | Description |
|--------|------|-------------|
| GET | `/admin/users` | List users |
| GET | `/admin/users/{id}` | User detail |
| POST | `/admin/users/{id}` | Update user |
| DELETE | `/admin/users/{id}` | Delete user |
| GET | `/admin/invites` | List invites |
| POST | `/admin/invites` | Create an invite |
| DELETE | `/admin/invites/{id}` | Delete an invite |

## Template Functions

The auth app contributes these template functions:

**Via `HasRequestFuncMap` (request-scoped):**

| Function | Example | Description |
|----------|---------|-------------|
| `currentUser` | `{{ if $u := currentUser }}{{ $u.Email }}{{ end }}` | Returns the authenticated `*auth.User` or `nil` |
| `isAuthenticated` | `{{ if isAuthenticated }}Sign out{{ else }}Sign in{{ end }}` | Returns `true` if a user is logged in |

These are available in all templates and are commonly used in layout navigation.

## Middleware

The auth app provides three middleware functions:

### Automatic User Loading

Registered automatically — loads the user from the session on every request:

```go
// In any handler, after auth middleware runs:
user := auth.GetUser(r)  // *auth.User or nil
```

### RequireAuth

Redirects unauthenticated users to the login page:

```go
r.Route("/notes", func(r chi.Router) {
    r.Use(auth.RequireAuth())
    // ... routes
})
```

The original URL is preserved via a `?next=` parameter for post-login redirect.

### RequireAdmin

Returns 403 if the user is not an admin:

```go
r.Route("/admin", func(r chi.Router) {
    r.Use(auth.RequireAuth(), auth.RequireAdmin())
    // ... routes
})
```

## Context Helpers

In Go code:

```go
user := auth.GetUser(r)    // *auth.User or nil
if auth.IsAuthenticated(r) { ... }
```

In templates (via `HasRequestFuncMap`):

```html
{{ if isAuthenticated }}
    <span>Hello, {{ (currentUser).Username }}</span>
{{ end }}
```

## Renderer

The auth app accepts a `Renderer` interface for user-facing HTML pages:

```go
type Renderer interface {
    RegisterPage(w http.ResponseWriter, r *http.Request, useEmail, inviteOnly bool, email, invite string) error
    LoginPage(w http.ResponseWriter, r *http.Request, loginRedirect string) error
    CredentialsPage(w http.ResponseWriter, r *http.Request, creds []Credential) error
    RecoveryPage(w http.ResponseWriter, r *http.Request, loginRedirect string) error
    RecoveryCodesPage(w http.ResponseWriter, r *http.Request, codes []string) error
    VerifyPendingPage(w http.ResponseWriter, r *http.Request) error
    VerifyEmailSuccess(w http.ResponseWriter, r *http.Request) error
    VerifyEmailError(w http.ResponseWriter, r *http.Request, errorCode string) error
}
```

## Admin Renderer

The `AdminRenderer` interface covers admin-only pages (user management, invites):

```go
type AdminRenderer interface {
    AdminUsersPage(w http.ResponseWriter, r *http.Request, users []User) error
    AdminUserDetailPage(w http.ResponseWriter, r *http.Request, user *User) error
    AdminInvitesPage(w http.ResponseWriter, r *http.Request, invites []Invite, createdURL string, useEmail bool) error
}
```

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--auth-login-redirect` | `AUTH_LOGIN_REDIRECT` | `/` | Redirect after login |
| `--auth-logout-redirect` | `AUTH_LOGOUT_REDIRECT` | `/auth/login` | Redirect after logout |
| `--auth-use-email` | `AUTH_USE_EMAIL` | `false` | Use email instead of username |
| `--auth-require-verification` | `AUTH_REQUIRE_VERIFICATION` | `false` | Require email verification |
| `--auth-invite-only` | `AUTH_INVITE_ONLY` | `false` | Require invite to register |
| `--webauthn-rp-id` | `WEBAUTHN_RP_ID` | `localhost` | WebAuthn Relying Party ID (domain) |
| `--webauthn-rp-display-name` | `WEBAUTHN_RP_DISPLAY_NAME` | `Web App` | WebAuthn RP display name |
| `--webauthn-rp-origin` | `WEBAUTHN_RP_ORIGIN` | (base URL) | WebAuthn RP origin |

## Email Service

For email verification and invite emails, wire up the [authmail](authmail.md) SMTP app or implement the `EmailService` interface:

```go
type EmailService interface {
    GenerateToken() (string, string, time.Time, error)
    SendVerification(ctx context.Context, toEmail, token string) error
    SendInvite(ctx context.Context, toEmail, token string) error
}
```

## Internationalisation

The auth app implements `HasTranslations` and ships English and German translations for all user-facing strings. When the `i18n` contrib app is registered, translations are auto-discovered and loaded.

Without the i18n app, templates fall back to displaying translation keys (which match their English text). To add a custom language, create a TOML file (e.g., `active.fr.toml`) with the same keys as `active.en.toml` and load it via `i18n.AddTranslations()`.

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `Migratable` | User, credential, recovery code, invite tables |
| `HasRoutes` | Auth routes |
| `HasMiddleware` | User loading from session |
| `HasAdmin` | Admin user/invite management routes and nav items |
| `HasStaticFiles` | Contributes embedded `webauthn.js` under `"auth"` prefix |
| `HasTemplates` | Contributes auth HTML templates |
| `HasRequestFuncMap` | Provides `currentUser`, `isAuthenticated` to templates |
| `HasTranslations` | Contributes English and German translation files |
| `Configurable` | Auth and WebAuthn flags |
| `HasDependencies` | Requires `session` |
