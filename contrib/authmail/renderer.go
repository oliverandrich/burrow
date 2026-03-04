// Package authmail defines the Renderer interface for auth email templates.
// Implementations live in sub-packages (e.g. authmail/smtp).
package authmail

// Renderer renders email content for verification and invite emails.
// Each email type has separate methods for HTML and plain text rendering,
// allowing independent customization of each format.
type Renderer interface {
	RenderVerificationHTML(verifyURL string) (subject, htmlBody string, err error)
	RenderVerificationText(verifyURL string) (subject, textBody string, err error)
	RenderInviteHTML(inviteURL string) (subject, htmlBody string, err error)
	RenderInviteText(inviteURL string) (subject, textBody string, err error)
}
