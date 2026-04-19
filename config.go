package burrow

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"
)

// Config holds core framework configuration.
type Config struct { //nolint:govet // fieldalignment: readability over optimization
	TLS      TLSConfig
	Database DatabaseConfig
	Storage  StorageConfig
	Server   ServerConfig
	I18n     I18nConfig
}

// I18nConfig holds internationalization settings.
type I18nConfig struct {
	DefaultLanguage    string
	SupportedLanguages string // comma-separated
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host            string
	BaseURL         string
	PIDFile         string
	Port            int
	MaxBodySize     int // in MB
	ShutdownTimeout int // in seconds
	Seed            bool
}

// DatabaseConfig holds database settings.
type DatabaseConfig struct {
	DSN string
}

// StorageConfig holds file-storage settings for attachments. Burrow
// constructs a den.Storage from these at boot and installs it on the
// opened den.DB via den.WithStorage, so domain apps can reach it via
// cfg.DB.Storage() and templates via the built-in mediaURL function.
// Set DSN to an empty string to disable Storage entirely.
type StorageConfig struct {
	// DSN selects the backend. Format: "<scheme>://<location>".
	// Supported schemes:
	//   - file:// — local filesystem. SQLAlchemy/JDBC convention:
	//     "file:///relative" (3 slashes) or "file:////absolute"
	//     (4 slashes). One leading slash is stripped on parse.
	// Default: file:///data/media (relative).
	DSN string
	// URLPrefix is the public-URL prefix applied to attachments by local
	// backends. Default: /media/.
	URLPrefix string
}

// TLSConfig holds TLS settings.
type TLSConfig struct {
	Mode     string // auto, acme, selfsigned, manual, off
	CertDir  string
	Email    string
	CertFile string
	KeyFile  string
}

// NewConfig creates a Config from a parsed CLI command.
func NewConfig(cmd *cli.Command) *Config {
	return &Config{
		Server: ServerConfig{
			Host:            cmd.String("host"),
			Port:            int(cmd.Int("port")),
			BaseURL:         cmd.String("base-url"),
			PIDFile:         cmd.String("pid-file"),
			MaxBodySize:     int(cmd.Int("max-body-size")),
			ShutdownTimeout: int(cmd.Int("shutdown-timeout")),
			Seed:            cmd.Bool("seed"),
		},
		Database: DatabaseConfig{
			DSN: cmd.String("database-dsn"),
		},
		Storage: StorageConfig{
			DSN:       cmd.String("storage-dsn"),
			URLPrefix: cmd.String("media-url-prefix"),
		},
		TLS: TLSConfig{
			Mode:     cmd.String("tls-mode"),
			CertDir:  cmd.String("tls-cert-dir"),
			Email:    cmd.String("tls-email"),
			CertFile: cmd.String("tls-cert-file"),
			KeyFile:  cmd.String("tls-key-file"),
		},
		I18n: I18nConfig{
			DefaultLanguage:    cmd.String("i18n-default-language"),
			SupportedLanguages: cmd.String("i18n-supported-languages"),
		},
	}
}

// IsHTTPS reports whether the base URL uses HTTPS.
func (c *Config) IsHTTPS() bool {
	return strings.HasPrefix(c.Server.BaseURL, "https://")
}

// ResolveBaseURL computes the base URL from server and TLS config
// if BaseURL is not explicitly set.
func (c *Config) ResolveBaseURL() string {
	if c.Server.BaseURL != "" {
		return c.Server.BaseURL
	}

	host := c.Server.Host
	port := c.Server.Port
	mode := strings.ToLower(c.TLS.Mode)
	useTLS := shouldUseTLS(mode, host)

	scheme := "http"
	if useTLS {
		scheme = "https"
	}

	if mode == "acme" {
		return fmt.Sprintf("https://%s", host)
	}

	if (scheme == "http" && port == 80) || (scheme == "https" && port == 443) {
		return fmt.Sprintf("%s://%s", scheme, host)
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, port)
}

// ValidateTLS checks that the TLS configuration is consistent.
// Call this early (before opening the database) to fail fast on misconfigurations.
func (c *Config) ValidateTLS(cmd *cli.Command) error {
	mode := c.resolvedTLSMode()

	switch mode {
	case "off", "selfsigned":
		return nil
	case "manual":
		if c.TLS.CertFile == "" {
			return fmt.Errorf("manual TLS mode requires --tls-cert-file")
		}
		if c.TLS.KeyFile == "" {
			return fmt.Errorf("manual TLS mode requires --tls-key-file")
		}
		return nil
	case "acme":
		if cmd.IsSet("port") {
			return fmt.Errorf("ACME mode uses ports 443/80; do not set --port explicitly")
		}
		return nil
	default:
		return fmt.Errorf("unknown TLS mode: %q", c.TLS.Mode)
	}
}

// resolvedTLSMode returns the effective TLS mode after resolving "auto".
func (c *Config) resolvedTLSMode() string {
	mode := strings.ToLower(c.TLS.Mode)
	if mode == "" || mode == "auto" {
		if IsLocalhost(c.Server.Host) {
			return "off"
		}
		return "acme"
	}
	return mode
}

func shouldUseTLS(mode, host string) bool {
	switch mode {
	case "off":
		return false
	case "acme", "selfsigned", "manual":
		return true
	default: // "auto" or empty
		return !IsLocalhost(host)
	}
}

// IsLocalhost checks if the host is a localhost address.
func IsLocalhost(host string) bool {
	switch host {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	}
	return strings.HasSuffix(host, ".localhost")
}

// FlagSources builds a cli.ValueSourceChain from an environment variable
// and an optional TOML key. If configSource is nil, only the env var is used.
// This is the standard way for contrib apps to wire up flag sources:
//
//	src := burrow.FlagSources(configSource, "MY_ENV_VAR", "app.toml_key")
func FlagSources(configSource func(key string) cli.ValueSource, envVar, tomlKey string) cli.ValueSourceChain {
	sources := []cli.ValueSource{cli.EnvVar(envVar)}
	if configSource != nil {
		sources = append(sources, configSource(tomlKey))
	}
	return cli.NewValueSourceChain(sources...)
}

// CoreFlags returns the CLI flags for core framework configuration.
// If configSource is provided, it is used as an additional value source
// (e.g. a TOML file sourcer) for each flag.
func CoreFlags(configSource func(key string) cli.ValueSource) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "host",
			Value:   "localhost",
			Usage:   "Host to bind to",
			Sources: FlagSources(configSource, "HOST", "server.host"),
		},
		&cli.IntFlag{
			Name:    "port",
			Value:   8080,
			Usage:   "Port to listen on",
			Sources: FlagSources(configSource, "PORT", "server.port"),
		},
		&cli.StringFlag{
			Name:    "base-url",
			Usage:   "Base URL for the application",
			Sources: FlagSources(configSource, "BASE_URL", "server.base_url"),
		},
		&cli.IntFlag{
			Name:    "max-body-size",
			Value:   1,
			Usage:   "Maximum request body size in MB",
			Sources: FlagSources(configSource, "MAX_BODY_SIZE", "server.max_body_size"),
		},
		&cli.StringFlag{
			Name:    "pid-file",
			Usage:   "Path to PID file (for systemd/supervisor integration)",
			Sources: FlagSources(configSource, "PID_FILE", "server.pid_file"),
		},
		&cli.IntFlag{
			Name:    "shutdown-timeout",
			Value:   10,
			Usage:   "Graceful shutdown timeout in seconds",
			Sources: FlagSources(configSource, "SHUTDOWN_TIMEOUT", "server.shutdown_timeout"),
		},
		&cli.BoolFlag{
			Name:    "seed",
			Usage:   "Run seed functions for all Seedable apps before starting the server",
			Sources: FlagSources(configSource, "SEED", "server.seed"),
		},
		&cli.StringFlag{
			Name:    "database-dsn",
			Value:   "sqlite:///data/app.db",
			Usage:   "Database URL (sqlite:///path or postgres://host/db)", //nolint:gosec // example URL, not a credential
			Sources: FlagSources(configSource, "DATABASE_DSN", "database.dsn"),
		},
		&cli.StringFlag{
			Name:    "storage-dsn",
			Value:   "file:///data/media",
			Usage:   "Storage URL for attachments (file:///relative or file:////absolute; empty disables Storage)",
			Sources: FlagSources(configSource, "STORAGE_DSN", "storage.dsn"),
		},
		&cli.StringFlag{
			Name:    "media-url-prefix",
			Value:   "/media/",
			Usage:   "Public URL prefix for attachments served through the local Storage backend",
			Sources: FlagSources(configSource, "MEDIA_URL_PREFIX", "storage.url_prefix"),
		},
		&cli.StringFlag{
			Name:    "tls-mode",
			Value:   "auto",
			Usage:   "TLS mode (auto, acme, selfsigned, manual, off)",
			Sources: FlagSources(configSource, "TLS_MODE", "tls.mode"),
		},
		&cli.StringFlag{
			Name:    "tls-cert-dir",
			Value:   "./data/certs",
			Usage:   "Directory for auto-generated certificates",
			Sources: FlagSources(configSource, "TLS_CERT_DIR", "tls.cert_dir"),
		},
		&cli.StringFlag{
			Name:    "tls-email",
			Usage:   "Email for ACME/Let's Encrypt registration",
			Sources: FlagSources(configSource, "TLS_EMAIL", "tls.email"),
		},
		&cli.StringFlag{
			Name:    "tls-cert-file",
			Usage:   "Path to TLS certificate file (manual mode)",
			Sources: FlagSources(configSource, "TLS_CERT_FILE", "tls.cert_file"),
		},
		&cli.StringFlag{
			Name:    "tls-key-file",
			Usage:   "Path to TLS private key file (manual mode)",
			Sources: FlagSources(configSource, "TLS_KEY_FILE", "tls.key_file"),
		},
		&cli.StringFlag{
			Name:    "i18n-default-language",
			Value:   "en",
			Usage:   "Default language",
			Sources: FlagSources(configSource, "I18N_DEFAULT_LANGUAGE", "i18n.default_language"),
		},
		&cli.StringFlag{
			Name:    "i18n-supported-languages",
			Value:   "en,de",
			Usage:   "Comma-separated supported languages",
			Sources: FlagSources(configSource, "I18N_SUPPORTED_LANGUAGES", "i18n.supported_languages"),
		},
	}
}
