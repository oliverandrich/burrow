package smtpmail

import (
	"fmt"
	"net/mail"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/authmail"
	"github.com/urfave/cli/v3"
)

// decorateFrom prefixes a bare SMTP From address with appName so the
// rendered "From" header reads "AppName <user@example.com>" instead of a
// raw address. If from is already in "Name <addr>" form (parsed name
// non-empty) or is unparseable, the input is returned unchanged.
func decorateFrom(from, appName string) string {
	if appName == "" || from == "" {
		return from
	}
	addr, err := mail.ParseAddress(from)
	if err != nil || addr == nil || addr.Name != "" {
		return from
	}
	return fmt.Sprintf("%s <%s>", appName, addr.Address)
}

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

func (a *App) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "smtp-host",
			Value:   "localhost",
			Usage:   "SMTP server host",
			Sources: burrow.FlagSources(configSource, "SMTP_HOST", "smtp.host"),
		},
		&cli.IntFlag{
			Name:    "smtp-port",
			Value:   587,
			Usage:   "SMTP server port",
			Sources: burrow.FlagSources(configSource, "SMTP_PORT", "smtp.port"),
		},
		&cli.StringFlag{
			Name:    "smtp-username",
			Usage:   "SMTP username",
			Sources: burrow.FlagSources(configSource, "SMTP_USERNAME", "smtp.username"),
		},
		&cli.StringFlag{
			Name:    "smtp-password",
			Usage:   "SMTP password",
			Sources: burrow.FlagSources(configSource, "SMTP_PASSWORD", "smtp.password"),
		},
		&cli.StringFlag{
			Name:    "smtp-from",
			Value:   "noreply@localhost",
			Usage:   "Sender email address",
			Sources: burrow.FlagSources(configSource, "SMTP_FROM", "smtp.from"),
		},
		&cli.StringFlag{
			Name:    "smtp-tls",
			Value:   "starttls",
			Usage:   "TLS mode: starttls, tls, or none",
			Sources: burrow.FlagSources(configSource, "SMTP_TLS", "smtp.tls"),
		},
	}
}

func (a *App) Configure(cfg *burrow.AppConfig, cmd *cli.Command) error {
	from := cmd.String("smtp-from")
	if cfg != nil && cfg.Config != nil {
		from = decorateFrom(from, cfg.Config.Server.AppName)
	}
	config := SMTPConfig{
		Host:     cmd.String("smtp-host"),
		Port:     int(cmd.Int("smtp-port")),
		Username: cmd.String("smtp-username"),
		Password: cmd.String("smtp-password"),
		From:     from,
		TLS:      cmd.String("smtp-tls"),
	}

	a.mailer = NewMailer(config, a.renderer)
	return nil
}

// Mailer returns the configured Mailer. Only valid after Configure.
func (a *App) Mailer() *Mailer {
	return a.mailer
}
