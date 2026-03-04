// Package authmail defines the Renderer interface for auth email templates.
// Implementations live in sub-packages (e.g. authmail/smtpmail).
package authmail

import "context"

// Renderer renders email content for verification and invite emails.
// Each email type has separate methods for HTML and plain text rendering,
// allowing independent customization of each format.
//
// The context carries the request locale (via i18n middleware), enabling
// localized email content. For verification emails the active user's locale
// is available; for invite emails the inviting admin's locale is passed.
type Renderer interface {
	RenderVerificationHTML(ctx context.Context, verifyURL string) (subject, htmlBody string, err error)
	RenderVerificationText(ctx context.Context, verifyURL string) (subject, textBody string, err error)
	RenderInviteHTML(ctx context.Context, inviteURL string) (subject, htmlBody string, err error)
	RenderInviteText(ctx context.Context, inviteURL string) (subject, textBody string, err error)
}
