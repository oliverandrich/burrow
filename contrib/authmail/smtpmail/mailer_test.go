package smtpmail

import (
	"context"
	"errors"
	"testing"

	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface check.
var _ auth.EmailService = (*Mailer)(nil)

type stubRenderer struct {
	verifyURL string
	inviteURL string
}

func (s *stubRenderer) RenderVerificationHTML(_ context.Context, verifyURL string) (string, string, error) {
	s.verifyURL = verifyURL
	return "Verify Subject", "<h1>Verify</h1>", nil
}

func (s *stubRenderer) RenderVerificationText(_ context.Context, verifyURL string) (string, string, error) {
	return "Verify Subject", "Verify: " + verifyURL, nil
}

func (s *stubRenderer) RenderInviteHTML(_ context.Context, inviteURL string) (string, string, error) {
	s.inviteURL = inviteURL
	return "Invite Subject", "<h1>Invite</h1>", nil
}

func (s *stubRenderer) RenderInviteText(_ context.Context, inviteURL string) (string, string, error) {
	return "Invite Subject", "Invite: " + inviteURL, nil
}

func TestMailerSendVerificationCallsRenderer(t *testing.T) {
	renderer := &stubRenderer{}
	m := NewMailer(SMTPConfig{
		Host: "localhost",
		Port: 2525,
		From: "test@example.com",
		TLS:  "none",
	}, renderer)

	// Will fail on dial (no SMTP server), but renderer should be called.
	_ = m.SendVerification(context.Background(), "user@example.com", "https://example.com/verify?token=abc")
	assert.Equal(t, "https://example.com/verify?token=abc", renderer.verifyURL)
}

func TestMailerSendInviteCallsRenderer(t *testing.T) {
	renderer := &stubRenderer{}
	m := NewMailer(SMTPConfig{
		Host: "localhost",
		Port: 2525,
		From: "test@example.com",
		TLS:  "none",
	}, renderer)

	_ = m.SendInvite(context.Background(), "user@example.com", "https://example.com/register?invite=abc")
	assert.Equal(t, "https://example.com/register?invite=abc", renderer.inviteURL)
}

func TestNewClientTLSModes(t *testing.T) {
	tests := []struct {
		name string
		tls  string
	}{
		{"starttls", "starttls"},
		{"implicit tls", "tls"},
		{"no tls", "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMailer(SMTPConfig{
				Host: "localhost",
				Port: 587,
				From: "test@example.com",
				TLS:  tt.tls,
			}, &stubRenderer{})

			c, err := m.newClient()
			require.NoError(t, err)
			assert.NotNil(t, c)
		})
	}
}

// errorRenderer returns errors from the specified render methods.
type errorRenderer struct {
	verifyHTMLErr error
	verifyTextErr error
	inviteHTMLErr error
	inviteTextErr error
}

func (e *errorRenderer) RenderVerificationHTML(_ context.Context, _ string) (string, string, error) {
	if e.verifyHTMLErr != nil {
		return "", "", e.verifyHTMLErr
	}
	return "Subject", "<h1>Verify</h1>", nil
}

func (e *errorRenderer) RenderVerificationText(_ context.Context, _ string) (string, string, error) {
	if e.verifyTextErr != nil {
		return "", "", e.verifyTextErr
	}
	return "Subject", "Verify", nil
}

func (e *errorRenderer) RenderInviteHTML(_ context.Context, _ string) (string, string, error) {
	if e.inviteHTMLErr != nil {
		return "", "", e.inviteHTMLErr
	}
	return "Subject", "<h1>Invite</h1>", nil
}

func (e *errorRenderer) RenderInviteText(_ context.Context, _ string) (string, string, error) {
	if e.inviteTextErr != nil {
		return "", "", e.inviteTextErr
	}
	return "Subject", "Invite", nil
}

func TestSendVerificationRendererHTMLError(t *testing.T) {
	m := NewMailer(SMTPConfig{
		Host: "localhost",
		Port: 2525,
		From: "test@example.com",
		TLS:  "none",
	}, &errorRenderer{verifyHTMLErr: errors.New("html render failed")})

	err := m.SendVerification(context.Background(), "user@example.com", "https://example.com/verify")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "render verification html")
	assert.Contains(t, err.Error(), "html render failed")
}

func TestSendVerificationRendererTextError(t *testing.T) {
	m := NewMailer(SMTPConfig{
		Host: "localhost",
		Port: 2525,
		From: "test@example.com",
		TLS:  "none",
	}, &errorRenderer{verifyTextErr: errors.New("text render failed")})

	err := m.SendVerification(context.Background(), "user@example.com", "https://example.com/verify")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "render verification text")
	assert.Contains(t, err.Error(), "text render failed")
}

func TestSendInviteRendererHTMLError(t *testing.T) {
	m := NewMailer(SMTPConfig{
		Host: "localhost",
		Port: 2525,
		From: "test@example.com",
		TLS:  "none",
	}, &errorRenderer{inviteHTMLErr: errors.New("html render failed")})

	err := m.SendInvite(context.Background(), "user@example.com", "https://example.com/invite")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "render invite html")
	assert.Contains(t, err.Error(), "html render failed")
}

func TestSendInviteRendererTextError(t *testing.T) {
	m := NewMailer(SMTPConfig{
		Host: "localhost",
		Port: 2525,
		From: "test@example.com",
		TLS:  "none",
	}, &errorRenderer{inviteTextErr: errors.New("text render failed")})

	err := m.SendInvite(context.Background(), "user@example.com", "https://example.com/invite")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "render invite text")
	assert.Contains(t, err.Error(), "text render failed")
}

func TestNewClientWithAuth(t *testing.T) {
	m := NewMailer(SMTPConfig{
		Host:     "smtp.example.com",
		Port:     587,
		Username: "user",
		Password: "pass",
		From:     "test@example.com",
		TLS:      "starttls",
	}, &stubRenderer{})

	c, err := m.newClient()
	require.NoError(t, err)
	assert.NotNil(t, c)
}
