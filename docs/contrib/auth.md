# Auth

WebAuthn (passkey) authentication with recovery codes, email verification, and invite-only registration.

**Package:** `github.com/oliverandrich/burrow/contrib/auth`

**Depends on:** `session`, `i18n`, `csrf`, `staticfiles`, `bootstrap` (for the default templates)

## Setup

```go
srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    auth.New(),
    bootstrap.New(),
    htmx.New(),
    admin.New(),
    staticApp, // staticfiles.New(emptyFS) — returns (*App, error)
    // ... other apps
)
```

`auth.New()` uses built-in defaults for the renderer and auth layout. Use options to override with custom implementations:

```go
// With custom renderer and layout.
auth.New(
    auth.WithRenderer(myCustomRenderer),
    auth.WithAuthLayout("myapp/auth-layout"),
)
```

## Default Templates

The auth app ships HTML templates via `HasTemplates`. These templates use the global template set and are rendered with `burrow.Render()`. The auth app also implements `HasRequestFuncMap` to provide `currentUser`, `isAuthenticated`, and other request-scoped functions available in all templates.

**Note:** When using default templates, register the `staticfiles` app so that `webauthn.js` is served. The auth app implements `HasStaticFiles` and contributes its assets under the `"auth"` prefix automatically.

## Auth Layout

Public auth pages (login, register, recovery, email verification) use a separate layout — typically a minimal page without the full app navigation. This avoids showing the navbar to unauthenticated users. Authenticated routes (`/auth/credentials`, `/auth/recovery-codes`) continue to use the global app layout.

By default, `auth.New()` uses a built-in layout template name (`DefaultAuthLayout()` returns `"auth/layout"`) that renders a minimal page with Bootstrap CSS. Override it with `auth.WithAuthLayout()`:

```go
auth.New(
    auth.WithAuthLayout("myapp/auth-layout"),
)
```

An auth layout is simply a template name string referring to a template in the global template set:

```html
{{ define "myapp/auth-layout" -}}
<!DOCTYPE html>
<html lang="{{ lang }}">
<head>
    <meta charset="utf-8">
    <title>{{ .Title }}</title>
    {{ template "bootstrap/css" . }}
</head>
<body class="d-flex align-items-center min-vh-100">
    <div class="container" style="max-width: 480px;">
        {{ .Content }}
    </div>
</body>
</html>
{{- end }}
```

See [Layouts & Rendering](../guide/layouts.md) for more details on how layouts work.

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
user := auth.CurrentUser(r.Context())  // *auth.User or nil
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
user := auth.CurrentUser(r.Context())    // *auth.User or nil
if auth.IsAuthenticated(r.Context()) { ... }
```

In templates (via `HasRequestFuncMap`):

```html
{{ if isAuthenticated }}
    <span>Hello, {{ (currentUser).Username }}</span>
{{ end }}
```

## Renderer

The auth app uses a `Renderer` interface to render all user-facing HTML pages. Each method corresponds to one page in the authentication flow.

### Default Renderer

By default, `auth.New()` uses a built-in renderer that calls `burrow.Render()` with the shipped `auth/*` templates. These templates use Bootstrap CSS and are wrapped in either a centered layout (login) or a card layout (register, credentials, recovery codes, etc.).

For most applications, the default renderer works out of the box — you only need to override it if you want to fundamentally change how auth pages are rendered.

### Custom Renderer

To fully control the auth page markup, implement the `Renderer` interface and pass it via `auth.WithRenderer()`:

```go
auth.New(
    auth.WithRenderer(myRenderer),
)
```

```go
type Renderer interface {
    RegisterPage(w http.ResponseWriter, r *http.Request, useEmail, inviteOnly bool, email, invite string) error
    LoginPage(w http.ResponseWriter, r *http.Request, loginRedirect string) error
    CredentialsPage(w http.ResponseWriter, r *http.Request, creds []Credential) error
    RecoveryPage(w http.ResponseWriter, r *http.Request, loginRedirect string) error
    RecoveryCodesPage(w http.ResponseWriter, r *http.Request, codes []string) error
    VerifyPendingPage(w http.ResponseWriter, r *http.Request) error
    VerifyEmailSuccessPage(w http.ResponseWriter, r *http.Request) error
    VerifyEmailErrorPage(w http.ResponseWriter, r *http.Request, errorCode string) error
}
```

Each method writes a complete HTTP response. You can use `burrow.Render()` with your own template names, or write HTML directly — whatever fits your application.

A minimal custom renderer might look like this:

```go
type myRenderer struct{}

func (r *myRenderer) LoginPage(w http.ResponseWriter, req *http.Request, loginRedirect string) error {
    return burrow.Render(w, req, http.StatusOK, "myapp/login", map[string]any{
        "LoginRedirect": loginRedirect,
    })
}

// ... implement the remaining methods
```

!!! tip
    You don't need a custom renderer just to change styles. The default templates use Bootstrap classes and are wrapped in the [auth layout](#auth-layout), which you can override separately via `auth.WithAuthLayout()`.

## Admin Integration

The auth app implements `HasAdmin` to provide user and invite management in the admin panel. It uses `modeladmin.ModelAdmin` internally — there is no separate `AdminRenderer` interface to implement.

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
    auth.New(
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
| `burrow.App` | Required: `Name()`, `Register()` |
| `Migratable` | User, credential, recovery code, invite tables |
| `HasRoutes` | Auth routes |
| `HasMiddleware` | User loading from session |
| `HasAdmin` | Admin user/invite management routes and nav items |
| `HasStaticFiles` | Contributes embedded `webauthn.js` under `"auth"` prefix |
| `HasTemplates` | Contributes auth HTML templates |
| `HasRequestFuncMap` | Provides `currentUser`, `isAuthenticated` to templates |
| `HasTranslations` | Contributes English and German translation files |
| `HasFuncMap` | Provides `credName`, `emailValue`, `deref` to templates |
| `Configurable` | Auth and WebAuthn flags |
| `HasDependencies` | Requires `session` |
| `HasJobs` | Registers `auth.send_email` job handler for email delivery via queue |
| `HasCLICommands` | Provides `promote`, `demote`, `create-invite` subcommands |
| `HasShutdown` | Stops the background WebAuthn challenge cleanup goroutine |
