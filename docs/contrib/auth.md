# Auth

WebAuthn (passkey) authentication with recovery codes, email verification, and invite-only registration.

**Package:** `codeberg.org/oliverandrich/burrow/contrib/auth`

**Depends on:** `session`

## Setup

```go
import authtpl "codeberg.org/oliverandrich/burrow/contrib/auth/templates"

srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    auth.New(
        auth.WithRenderer(authtpl.DefaultRenderer()),
        auth.WithAuthLayout(authtpl.AuthLayout()),
        auth.WithAdminRenderer(authtpl.DefaultAdminRenderer()),
    ),
    admin.New(
        admin.WithLayout(admintpl.Layout()),
        admin.WithDashboardRenderer(admintpl.DefaultDashboardRenderer()),
    ),
    staticfiles.New(emptyFS), // serves auth + admin static files
    // ... other apps
)
```

Or with custom renderers:

```go
// With custom templates.
auth.New(auth.WithRenderer(myCustomRenderer))

// API-only (no HTML pages).
auth.New()
```

## Default Templates

The auth app ships batteries-included Templ templates in the `auth/templates` sub-package. Use `authtpl.DefaultRenderer()` and `authtpl.DefaultAdminRenderer()` (imported as `authtpl "codeberg.org/oliverandrich/burrow/contrib/auth/templates"`) for ready-to-use pages.

The default templates:
- Read `burrow.Layout(ctx)` at render time — if a layout is set, content is wrapped in it
- Use the `staticfiles` system for the embedded `webauthn.js` (content-hashed URLs)
- Include inline JavaScript for WebAuthn browser ceremonies

**Note:** When using default templates, register the `staticfiles` app so that `webauthn.js` is served. The auth app implements `HasStaticFiles` and contributes its assets under the `"auth"` prefix automatically.

## Auth Layout

By default, public auth pages (login, register, recovery, email verification) render inside the global app layout — which typically includes a full navbar. This is often undesirable for unauthenticated users.

Use `auth.WithAuthLayout()` to override the layout for public auth pages. Authenticated routes (`/auth/credentials`, `/auth/recovery-codes`) continue to use the global app layout.

```go
auth.New(
    auth.WithRenderer(authtpl.DefaultRenderer()),
    auth.WithAuthLayout(authtpl.AuthLayout()),    // built-in minimal layout
    // auth.WithAuthLayout(myMinimalLayout),       // or your own custom layout
)
```

The built-in `authtpl.AuthLayout()` renders a minimal HTML shell with Bootstrap CSS but no navigation — just a clean, centered page suitable for login and registration forms.

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
| GET | `/auth/recovery-codes` | View recovery codes after registration/regeneration (auth required) |
| POST | `/auth/recovery-codes/ack` | Acknowledge recovery codes and clear from session (auth required) |
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

```go
// Get the current user from request context.
user := auth.GetUser(r)    // *auth.User or nil

// Check if a user is logged in.
if auth.IsAuthenticated(r) { ... }
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

Use `authtpl.DefaultRenderer()` (from the `auth/templates` sub-package) for built-in templates, or implement the interface to provide your own.

## Admin Renderer

The `AdminRenderer` interface covers admin-only pages (user management, invites):

```go
type AdminRenderer interface {
    AdminUsersPage(w http.ResponseWriter, r *http.Request, users []User) error
    AdminUserDetailPage(w http.ResponseWriter, r *http.Request, user *User) error
    AdminInvitesPage(w http.ResponseWriter, r *http.Request, invites []Invite, createdURL string, useEmail bool) error
}
```

Use `authtpl.DefaultAdminRenderer()` (from the `auth/templates` sub-package) for built-in templates, or implement the interface for custom admin pages.

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

For email verification and invite emails, set an email service after configuration:

```go
auth.New(
    auth.WithRenderer(authtpl.DefaultRenderer()),
    auth.WithEmailService(myEmailService),
)
```

The `EmailService` interface:

```go
type EmailService interface {
    GenerateToken() (string, string, time.Time, error)
    SendVerification(ctx context.Context, toEmail, token string) error
    SendInvite(ctx context.Context, toEmail, token string) error
}
```

## Static Files

The auth app embeds a WebAuthn JavaScript helper and implements `HasStaticFiles` to contribute it under the `"auth"` prefix:

| File | Description |
|------|-------------|
| `webauthn.js` | WebAuthn browser ceremony helpers (register, login, add credential) |

This is served at `/static/auth/webauthn.js` (with content-hashed URL) when the `staticfiles` app is registered.

## Internationalization

The auth app implements `HasTranslations` and ships English and German translations for all user-facing strings. When the `i18n` contrib app is registered, translations are auto-discovered and loaded.

```go
srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    i18n.New(),     // must come before auth for middleware order
    authApp,
    // ...
)
```

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
| `HasTranslations` | Contributes English and German translation files |
| `Configurable` | Auth and WebAuthn flags |
| `HasDependencies` | Requires `session` |
