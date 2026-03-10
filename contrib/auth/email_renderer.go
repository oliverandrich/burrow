package auth

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"

	"github.com/oliverandrich/burrow/contrib/authmail"
	"github.com/oliverandrich/burrow/i18n"
)

//go:embed email_templates/emails.html
var emailTemplateFS embed.FS

var emailTemplates = template.Must(template.ParseFS(emailTemplateFS, "email_templates/emails.html"))

type defaultEmailRenderer struct{}

// DefaultEmailRenderer returns a Renderer that uses the built-in HTML email
// templates with i18n support. When a localizer is present in the context
// (via i18n middleware or i18n.App.WithLocale), all strings are translated.
// Without a localizer, translation keys are returned as-is.
func DefaultEmailRenderer() authmail.Renderer {
	return &defaultEmailRenderer{}
}

func (d *defaultEmailRenderer) RenderVerificationHTML(ctx context.Context, verifyURL string) (string, string, error) {
	data := map[string]any{
		"URL":      verifyURL,
		"Heading":  i18n.T(ctx, "email-verification-heading"),
		"Body":     i18n.T(ctx, "email-verification-body"),
		"Button":   i18n.T(ctx, "email-verification-button"),
		"Fallback": i18n.T(ctx, "email-verification-fallback"),
	}
	html, err := renderEmailTemplate("authmail/verification", data)
	if err != nil {
		return "", "", fmt.Errorf("render verification html: %w", err)
	}
	return i18n.T(ctx, "email-verification-subject"), html, nil
}

func (d *defaultEmailRenderer) RenderVerificationText(ctx context.Context, verifyURL string) (string, string, error) {
	subject := i18n.T(ctx, "email-verification-subject")
	heading := i18n.T(ctx, "email-verification-heading")
	body := i18n.T(ctx, "email-verification-body")
	text := heading + "\n\n" + body + "\n" + verifyURL
	return subject, text, nil
}

func (d *defaultEmailRenderer) RenderInviteHTML(ctx context.Context, inviteURL string) (string, string, error) {
	data := map[string]any{
		"URL":      inviteURL,
		"Heading":  i18n.T(ctx, "email-invite-heading"),
		"Body":     i18n.T(ctx, "email-invite-body"),
		"Button":   i18n.T(ctx, "email-invite-button"),
		"Fallback": i18n.T(ctx, "email-invite-fallback"),
	}
	html, err := renderEmailTemplate("authmail/invite", data)
	if err != nil {
		return "", "", fmt.Errorf("render invite html: %w", err)
	}
	return i18n.T(ctx, "email-invite-subject"), html, nil
}

func (d *defaultEmailRenderer) RenderInviteText(ctx context.Context, inviteURL string) (string, string, error) {
	subject := i18n.T(ctx, "email-invite-subject")
	heading := i18n.T(ctx, "email-invite-heading")
	body := i18n.T(ctx, "email-invite-body")
	text := heading + "\n\n" + body + "\n" + inviteURL
	return subject, text, nil
}

func renderEmailTemplate(name string, data map[string]any) (string, error) {
	var buf bytes.Buffer
	if err := emailTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
