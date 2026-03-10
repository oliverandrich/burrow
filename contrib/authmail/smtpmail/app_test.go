package smtpmail

import (
	"context"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestAppName(t *testing.T) {
	a := New()
	assert.Equal(t, "authmail-smtp", a.Name())
}

func TestAppRegister(t *testing.T) {
	a := New()
	err := a.Register(&burrow.AppConfig{})
	require.NoError(t, err)
}

func TestAppFlags(t *testing.T) {
	a := New()
	flags := a.Flags(nil)
	assert.Len(t, flags, 6)
}

func TestAppWithRenderer(t *testing.T) {
	r := &stubRenderer{}
	a := New(WithRenderer(r))
	assert.NotNil(t, a.renderer)
}

func TestAppMailerNilBeforeConfigure(t *testing.T) {
	a := New()
	assert.Nil(t, a.Mailer())
}

func TestConfigureCreatesMailer(t *testing.T) {
	a := New(WithRenderer(&stubRenderer{}))

	cmd := &cli.Command{
		Name:  "test",
		Flags: a.Flags(nil),
		Action: func(_ context.Context, cmd *cli.Command) error {
			return a.Configure(cmd)
		},
	}
	err := cmd.Run(t.Context(), []string{"test"})
	require.NoError(t, err)

	m := a.Mailer()
	require.NotNil(t, m)
	assert.Equal(t, "localhost", m.config.Host)
	assert.Equal(t, 587, m.config.Port)
	assert.Equal(t, "noreply@localhost", m.config.From)
	assert.Equal(t, "starttls", m.config.TLS)
}

func TestConfigureWithCustomFlags(t *testing.T) {
	a := New(WithRenderer(&stubRenderer{}))

	cmd := &cli.Command{
		Name:  "test",
		Flags: a.Flags(nil),
		Action: func(_ context.Context, cmd *cli.Command) error {
			return a.Configure(cmd)
		},
	}
	err := cmd.Run(t.Context(), []string{
		"test",
		"--smtp-host", "mail.example.com",
		"--smtp-port", "465",
		"--smtp-username", "user",
		"--smtp-password", "secret",
		"--smtp-from", "noreply@example.com",
		"--smtp-tls", "tls",
	})
	require.NoError(t, err)

	m := a.Mailer()
	require.NotNil(t, m)
	assert.Equal(t, "mail.example.com", m.config.Host)
	assert.Equal(t, 465, m.config.Port)
	assert.Equal(t, "user", m.config.Username)
	assert.Equal(t, "secret", m.config.Password)
	assert.Equal(t, "noreply@example.com", m.config.From)
	assert.Equal(t, "tls", m.config.TLS)
}
