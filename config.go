package burrow

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"
)

// Config holds core framework configuration.
type Config struct {
	TLS      TLSConfig
	Database DatabaseConfig
	Server   ServerConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host            string
	BaseURL         string
	Port            int
	MaxBodySize     int // in MB
	ShutdownTimeout int // in seconds
}

// DatabaseConfig holds database settings.
type DatabaseConfig struct {
	DSN string
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
			MaxBodySize:     int(cmd.Int("max-body-size")),
			ShutdownTimeout: int(cmd.Int("shutdown-timeout")),
		},
		Database: DatabaseConfig{
			DSN: cmd.String("database-dsn"),
		},
		TLS: TLSConfig{
			Mode:     cmd.String("tls-mode"),
			CertDir:  cmd.String("tls-cert-dir"),
			Email:    cmd.String("tls-email"),
			CertFile: cmd.String("tls-cert-file"),
			KeyFile:  cmd.String("tls-key-file"),
		},
	}
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

// CoreFlags returns the CLI flags for core framework configuration.
// If configSource is provided, it is used as an additional value source
// (e.g. a TOML file sourcer) for each flag.
func CoreFlags(configSource func(key string) cli.ValueSource) []cli.Flag {
	src := func(envVar, tomlKey string) cli.ValueSourceChain {
		sources := []cli.ValueSource{cli.EnvVar(envVar)}
		if configSource != nil {
			sources = append(sources, configSource(tomlKey))
		}
		return cli.NewValueSourceChain(sources...)
	}

	return []cli.Flag{
		&cli.StringFlag{
			Name:    "host",
			Value:   "localhost",
			Usage:   "Host to bind to",
			Sources: src("HOST", "server.host"),
		},
		&cli.IntFlag{
			Name:    "port",
			Value:   8080,
			Usage:   "Port to listen on",
			Sources: src("PORT", "server.port"),
		},
		&cli.StringFlag{
			Name:    "base-url",
			Usage:   "Base URL for the application",
			Sources: src("BASE_URL", "server.base_url"),
		},
		&cli.IntFlag{
			Name:    "max-body-size",
			Value:   1,
			Usage:   "Maximum request body size in MB",
			Sources: src("MAX_BODY_SIZE", "server.max_body_size"),
		},
		&cli.IntFlag{
			Name:    "shutdown-timeout",
			Value:   10,
			Usage:   "Graceful shutdown timeout in seconds",
			Sources: src("SHUTDOWN_TIMEOUT", "server.shutdown_timeout"),
		},
		&cli.StringFlag{
			Name:    "database-dsn",
			Value:   "./data/app.db",
			Usage:   "Database DSN",
			Sources: src("DATABASE_DSN", "database.dsn"),
		},
		&cli.StringFlag{
			Name:    "tls-mode",
			Value:   "auto",
			Usage:   "TLS mode (auto, acme, selfsigned, manual, off)",
			Sources: src("TLS_MODE", "tls.mode"),
		},
		&cli.StringFlag{
			Name:    "tls-cert-dir",
			Value:   "./data/certs",
			Usage:   "Directory for auto-generated certificates",
			Sources: src("TLS_CERT_DIR", "tls.cert_dir"),
		},
		&cli.StringFlag{
			Name:    "tls-email",
			Usage:   "Email for ACME/Let's Encrypt registration",
			Sources: src("TLS_EMAIL", "tls.email"),
		},
		&cli.StringFlag{
			Name:    "tls-cert-file",
			Usage:   "Path to TLS certificate file (manual mode)",
			Sources: src("TLS_CERT_FILE", "tls.cert_file"),
		},
		&cli.StringFlag{
			Name:    "tls-key-file",
			Usage:   "Path to TLS private key file (manual mode)",
			Sources: src("TLS_KEY_FILE", "tls.key_file"),
		},
	}
}
