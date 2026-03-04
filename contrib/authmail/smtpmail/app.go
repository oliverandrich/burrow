package smtpmail

import (
	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/authmail"
	"github.com/urfave/cli/v3"
)

// App implements the authmail SMTP contrib app.
type App struct {
	renderer authmail.Renderer
	mailer   *Mailer
}

// Option configures the SMTP mail app.
type Option func(*App)

// WithRenderer sets a custom email renderer.
func WithRenderer(r authmail.Renderer) Option {
	return func(a *App) {
		a.renderer = r
	}
}

// New creates a new SMTP mail app.
func New(opts ...Option) *App {
	a := &App{}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *App) Name() string { return "authmail-smtp" }

func (a *App) Register(_ *burrow.AppConfig) error { return nil }

func (a *App) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "smtp-host",
			Value:   "localhost",
			Usage:   "SMTP server host",
			Sources: cli.EnvVars("SMTP_HOST"),
		},
		&cli.IntFlag{
			Name:    "smtp-port",
			Value:   587,
			Usage:   "SMTP server port",
			Sources: cli.EnvVars("SMTP_PORT"),
		},
		&cli.StringFlag{
			Name:    "smtp-username",
			Usage:   "SMTP username",
			Sources: cli.EnvVars("SMTP_USERNAME"),
		},
		&cli.StringFlag{
			Name:    "smtp-password",
			Usage:   "SMTP password",
			Sources: cli.EnvVars("SMTP_PASSWORD"),
		},
		&cli.StringFlag{
			Name:    "smtp-from",
			Value:   "noreply@localhost",
			Usage:   "Sender email address",
			Sources: cli.EnvVars("SMTP_FROM"),
		},
		&cli.StringFlag{
			Name:    "smtp-tls",
			Value:   "starttls",
			Usage:   "TLS mode: starttls, tls, or none",
			Sources: cli.EnvVars("SMTP_TLS"),
		},
	}
}

func (a *App) Configure(cmd *cli.Command) error {
	config := SMTPConfig{
		Host:     cmd.String("smtp-host"),
		Port:     int(cmd.Int("smtp-port")),
		Username: cmd.String("smtp-username"),
		Password: cmd.String("smtp-password"),
		From:     cmd.String("smtp-from"),
		TLS:      cmd.String("smtp-tls"),
	}

	a.mailer = NewMailer(config, a.renderer)
	return nil
}

// Mailer returns the configured Mailer. Only valid after Configure.
func (a *App) Mailer() *Mailer {
	return a.mailer
}

// Compile-time interface assertions.
var (
	_ burrow.App          = (*App)(nil)
	_ burrow.Configurable = (*App)(nil)
)
