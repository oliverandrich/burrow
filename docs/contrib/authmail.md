# Auth Mail

Pluggable email rendering and SMTP delivery for auth emails (verification and invites).

**Package:** `codeberg.org/oliverandrich/burrow/contrib/authmail`

**SMTP implementation:** `codeberg.org/oliverandrich/burrow/contrib/authmail/smtpmail`

## Overview

The `authmail` package defines the `Renderer` interface for email content. The `smtpmail` sub-package provides a concrete SMTP implementation that sends emails via any SMTP server.

## Setup

```go
import "codeberg.org/oliverandrich/burrow/contrib/authmail/smtpmail"

srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    auth.New(
        auth.WithRenderer(authRenderer),
        auth.WithEmailService(smtpApp.Mailer()), // wire after Configure
    ),
    smtpmail.New(
        smtpmail.WithRenderer(emailRenderer), // custom email templates
    ),
    // ... other apps
)
```

## Renderer Interface

The `Renderer` interface defines methods for rendering email content. Each email type has separate methods for HTML and plain text:

```go
type Renderer interface {
    RenderVerificationHTML(ctx context.Context, verifyURL string) (subject, htmlBody string, err error)
    RenderVerificationText(ctx context.Context, verifyURL string) (subject, textBody string, err error)
    RenderInviteHTML(ctx context.Context, inviteURL string) (subject, htmlBody string, err error)
    RenderInviteText(ctx context.Context, inviteURL string) (subject, textBody string, err error)
}
```

The context carries the request locale (via i18n middleware), enabling localised email content.

A default renderer implementation is available in the `smtpmail/templates` sub-package.

## SMTP App

The `smtpmail.App` is a burrow contrib app that configures an SMTP client from CLI flags:

```go
smtpApp := smtpmail.New()

// After Configure(), access the mailer:
mailer := smtpApp.Mailer()
```

The `Mailer` implements `auth.EmailService`, so it can be wired directly into the auth app.

### Email Delivery

The mailer sends multipart emails (HTML + plain text) using [go-mail](https://github.com/wneessen/go-mail):

```go
// Sends a verification email
mailer.SendVerification(ctx, "alice@example.com", "https://example.com/auth/verify-email?token=...")

// Sends an invite email
mailer.SendInvite(ctx, "bob@example.com", "https://example.com/auth/register?invite=...")
```

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--smtp-host` | `SMTP_HOST` | `localhost` | SMTP server host |
| `--smtp-port` | `SMTP_PORT` | `587` | SMTP server port |
| `--smtp-username` | `SMTP_USERNAME` | (none) | SMTP username |
| `--smtp-password` | `SMTP_PASSWORD` | (none) | SMTP password |
| `--smtp-from` | `SMTP_FROM` | `noreply@localhost` | Sender email address |
| `--smtp-tls` | `SMTP_TLS` | `starttls` | TLS mode: `starttls`, `tls`, or `none` |

### TLS Modes

| Mode | Description |
|------|-------------|
| `starttls` | Connect on port 587, upgrade to TLS via STARTTLS (recommended) |
| `tls` | Connect with implicit TLS on port 465 |
| `none` | No encryption (development only) |

## Interfaces Implemented

### smtpmail.App

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `Configurable` | SMTP configuration flags |
