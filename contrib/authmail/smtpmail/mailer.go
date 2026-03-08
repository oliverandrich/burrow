// Package smtpmail provides an SMTP-based implementation of auth.EmailService
// with pluggable email templates via authmail.Renderer.
package smtpmail

import (
	"context"
	"fmt"

	"github.com/oliverandrich/burrow/contrib/authmail"

	"github.com/wneessen/go-mail"
)

// SMTPConfig holds SMTP connection settings.
type SMTPConfig struct { //nolint:govet // fieldalignment: readability over optimization
	Host     string
	Port     int
	Username string
	Password string //nolint:gosec // G117: not a hardcoded credential, populated at runtime
	From     string
	TLS      string // "starttls" (default, port 587), "tls" (implicit, port 465), "none"
}

// Mailer sends emails via SMTP. It implements auth.EmailService.
type Mailer struct { //nolint:govet // fieldalignment: readability over optimization
	config   SMTPConfig
	renderer authmail.Renderer
}

// NewMailer creates a new Mailer with the given config and renderer.
func NewMailer(config SMTPConfig, renderer authmail.Renderer) *Mailer {
	return &Mailer{config: config, renderer: renderer}
}

// SendVerification sends a verification email with the given URL.
func (m *Mailer) SendVerification(ctx context.Context, toEmail, verifyURL string) error {
	subject, html, err := m.renderer.RenderVerificationHTML(ctx, verifyURL)
	if err != nil {
		return fmt.Errorf("render verification html: %w", err)
	}
	_, text, err := m.renderer.RenderVerificationText(ctx, verifyURL)
	if err != nil {
		return fmt.Errorf("render verification text: %w", err)
	}
	return m.send(ctx, toEmail, subject, html, text)
}

// SendInvite sends an invite email with the given URL.
func (m *Mailer) SendInvite(ctx context.Context, toEmail, inviteURL string) error {
	subject, html, err := m.renderer.RenderInviteHTML(ctx, inviteURL)
	if err != nil {
		return fmt.Errorf("render invite html: %w", err)
	}
	_, text, err := m.renderer.RenderInviteText(ctx, inviteURL)
	if err != nil {
		return fmt.Errorf("render invite text: %w", err)
	}
	return m.send(ctx, toEmail, subject, html, text)
}

func (m *Mailer) send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	msg := mail.NewMsg()
	if err := msg.From(m.config.From); err != nil {
		return fmt.Errorf("set from: %w", err)
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("set to: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, textBody)
	msg.AddAlternativeString(mail.TypeTextHTML, htmlBody)

	c, err := m.newClient()
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}
	if err := c.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

func (m *Mailer) newClient() (*mail.Client, error) {
	opts := []mail.Option{
		mail.WithPort(m.config.Port),
	}

	switch m.config.TLS {
	case "tls":
		opts = append(opts, mail.WithSSL())
	case "none":
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	default: // "starttls"
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	}

	if m.config.Username != "" {
		opts = append(opts,
			mail.WithUsername(m.config.Username),
			mail.WithPassword(m.config.Password),
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
		)
	}

	return mail.NewClient(m.config.Host, opts...)
}
