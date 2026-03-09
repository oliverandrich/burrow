// Package templates provides the default HTML email renderer for auth emails
// (verification, recovery). It embeds built-in email templates that are used
// when no custom authmail.Renderer is configured.
package templates

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"

	"github.com/oliverandrich/burrow/contrib/authmail"
)

//go:embed emails.html
var templateFS embed.FS

var emailTemplates = template.Must(template.ParseFS(templateFS, "emails.html"))

type defaultRenderer struct{}

// DefaultRenderer returns a Renderer that uses the built-in HTML email templates.
func DefaultRenderer() authmail.Renderer {
	return &defaultRenderer{}
}

func (d *defaultRenderer) RenderVerificationHTML(_ context.Context, verifyURL string) (string, string, error) {
	html, err := renderTemplate("authmail/verification", verifyURL)
	if err != nil {
		return "", "", fmt.Errorf("render verification html: %w", err)
	}
	return "Verify your email", html, nil
}

func (d *defaultRenderer) RenderVerificationText(_ context.Context, verifyURL string) (string, string, error) {
	text := "Verify your email\n\nClick the link below to verify your email address:\n" + verifyURL
	return "Verify your email", text, nil
}

func (d *defaultRenderer) RenderInviteHTML(_ context.Context, inviteURL string) (string, string, error) {
	html, err := renderTemplate("authmail/invite", inviteURL)
	if err != nil {
		return "", "", fmt.Errorf("render invite html: %w", err)
	}
	return "You've been invited", html, nil
}

func (d *defaultRenderer) RenderInviteText(_ context.Context, inviteURL string) (string, string, error) {
	text := "You've been invited\n\nYou've been invited to create an account. Click the link below to get started:\n" + inviteURL
	return "You've been invited", text, nil
}

func renderTemplate(name, url string) (string, error) {
	var buf bytes.Buffer
	if err := emailTemplates.ExecuteTemplate(&buf, name, map[string]any{"URL": url}); err != nil {
		return "", err
	}
	return buf.String(), nil
}
