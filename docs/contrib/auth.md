# Auth

WebAuthn (passkey) authentication with recovery codes, email verification, and invite-only registration.

**Package:** `codeberg.org/oliverandrich/go-webapp-template/contrib/auth`

**Depends on:** `session`

## Setup

```go
srv := core.NewServer(
    &session.App{},       // Must come first
    auth.New(renderer),   // Pass nil for API-only (no HTML pages)
    // ... other apps
)
```

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
| POST | `/auth/recovery-codes/regenerate` | Regenerate recovery codes (auth required) |
| GET | `/auth/verify-pending` | Email verification pending page |
| GET | `/auth/verify-email` | Verify email via token |
| POST | `/auth/resend-verification` | Resend verification email |

Admin-only routes (require auth + admin role):

| Method | Path | Description |
|--------|------|-------------|
| GET | `/admin/invites` | List invites |
| POST | `/admin/invites` | Create an invite |
| DELETE | `/admin/invites/:id` | Delete an invite |

## Middleware

The auth app provides three middleware functions:

### Automatic User Loading

Registered automatically — loads the user from the session on every request:

```go
// In any handler, after auth middleware runs:
user := auth.GetUser(c)  // *auth.User or nil
```

### RequireAuth

Redirects unauthenticated users to the login page:

```go
g := e.Group("/notes", auth.RequireAuth())
```

The original URL is preserved via a `?next=` parameter for post-login redirect.

### RequireAdmin

Returns 403 if the user is not an admin:

```go
admin := e.Group("/admin", auth.RequireAuth(), auth.RequireAdmin())
```

## Context Helpers

```go
// Get the current user from Echo context.
user := auth.GetUser(c)    // *auth.User or nil

// Check if a user is logged in.
if auth.IsAuthenticated(c) { ... }

// Set user in context (used by the middleware, rarely needed in app code).
auth.SetUser(c, user)
```

## Renderer

The auth app accepts a `Renderer` interface for HTML pages:

```go
type Renderer interface {
    RegisterPage(c *echo.Context, useEmail, inviteOnly bool, email, invite string) error
    LoginPage(c *echo.Context, loginRedirect string) error
    CredentialsPage(c *echo.Context, creds []Credential) error
    RecoveryPage(c *echo.Context, loginRedirect string) error
    VerifyPendingPage(c *echo.Context) error
    VerifyEmailSuccess(c *echo.Context) error
    VerifyEmailError(c *echo.Context, errorCode string) error
    InvitesPage(c *echo.Context, invites []Invite, createdURL string, useEmail bool) error
}
```

Pass `nil` to `auth.New(nil)` for API-only mode (returns JSON errors instead of rendering HTML pages). Implement the interface to provide your own registration/login templates with your CSS framework.

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--auth-login-redirect` | `AUTH_LOGIN_REDIRECT` | `/dashboard` | Redirect after login |
| `--auth-use-email` | `AUTH_USE_EMAIL` | `false` | Use email instead of username |
| `--auth-require-verification` | `AUTH_REQUIRE_VERIFICATION` | `false` | Require email verification |
| `--auth-invite-only` | `AUTH_INVITE_ONLY` | `false` | Require invite to register |
| `--webauthn-rp-id` | `WEBAUTHN_RP_ID` | `localhost` | WebAuthn Relying Party ID (domain) |
| `--webauthn-rp-display-name` | `WEBAUTHN_RP_DISPLAY_NAME` | `Web App` | WebAuthn RP display name |
| `--webauthn-rp-origin` | `WEBAUTHN_RP_ORIGIN` | (base URL) | WebAuthn RP origin |

## Email Service

For email verification and invite emails, set an email service after configuration:

```go
authApp := auth.New(renderer)
// ... after srv.Run boots the server ...
authApp.SetEmailService(myEmailService)
```

The `EmailService` interface:

```go
type EmailService interface {
    GenerateToken() (string, string, time.Time, error)
    SendVerification(ctx context.Context, toEmail, token string) error
    SendInvite(ctx context.Context, toEmail, token string) error
}
```

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `core.App` | Required: `Name()`, `Register()` |
| `Migratable` | User, credential, recovery code, invite tables |
| `HasRoutes` | Auth and invite routes |
| `HasMiddleware` | User loading from session |
| `Configurable` | Auth and WebAuthn flags |
| `HasDependencies` | Requires `session` |
