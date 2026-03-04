package templates

import (
	"bytes"
	"context"
	"fmt"

	"codeberg.org/oliverandrich/burrow/contrib/authmail"

	"github.com/a-h/templ"
)

type defaultRenderer struct{}

// DefaultRenderer returns a Renderer that uses the built-in Templ email templates.
func DefaultRenderer() authmail.Renderer {
	return &defaultRenderer{}
}

func (d *defaultRenderer) RenderVerificationHTML(ctx context.Context, verifyURL string) (string, string, error) {
	html, err := renderComponent(ctx, verificationEmail(verifyURL))
	if err != nil {
		return "", "", fmt.Errorf("render verification html: %w", err)
	}
	return "Verify your email", html, nil
}

func (d *defaultRenderer) RenderVerificationText(_ context.Context, verifyURL string) (string, string, error) {
	text := "Verify your email\n\nClick the link below to verify your email address:\n" + verifyURL
	return "Verify your email", text, nil
}

func (d *defaultRenderer) RenderInviteHTML(ctx context.Context, inviteURL string) (string, string, error) {
	html, err := renderComponent(ctx, inviteEmail(inviteURL))
	if err != nil {
		return "", "", fmt.Errorf("render invite html: %w", err)
	}
	return "You've been invited", html, nil
}

func (d *defaultRenderer) RenderInviteText(_ context.Context, inviteURL string) (string, string, error) {
	text := "You've been invited\n\nYou've been invited to create an account. Click the link below to get started:\n" + inviteURL
	return "You've been invited", text, nil
}

func renderComponent(ctx context.Context, c templ.Component) (string, error) {
	var buf bytes.Buffer
	if err := c.Render(ctx, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
