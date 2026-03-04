package smtpmail

import (
	"context"
	"testing"

	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface check.
var _ auth.EmailService = (*Mailer)(nil)

type stubRenderer struct {
	verifyURL string
	inviteURL string
}

func (s *stubRenderer) RenderVerificationHTML(verifyURL string) (string, string, error) {
	s.verifyURL = verifyURL
	return "Verify Subject", "<h1>Verify</h1>", nil
}

func (s *stubRenderer) RenderVerificationText(verifyURL string) (string, string, error) {
	return "Verify Subject", "Verify: " + verifyURL, nil
}

func (s *stubRenderer) RenderInviteHTML(inviteURL string) (string, string, error) {
	s.inviteURL = inviteURL
	return "Invite Subject", "<h1>Invite</h1>", nil
}

func (s *stubRenderer) RenderInviteText(inviteURL string) (string, string, error) {
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
