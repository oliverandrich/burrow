package burrow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
)

// Config holds core framework configuration.
type Config struct { //nolint:govet // fieldalignment: readability over optimization
	TLS      TLSConfig
	Database DatabaseConfig
	Storage  StorageConfig
	Server   ServerConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host            string
	BaseURL         string
	PIDFile         string
	AppName         string
	Port            int
	MaxBodySize     int // in MB
	ShutdownTimeout int // in seconds
}

// resolveAppName returns the explicit flag value when set, falling back to
// the binary basename (os.Args[0]). It only returns "" when both the flag
// and os.Args[0] are missing — a corner case in embedded contexts.
func resolveAppName(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if len(os.Args) == 0 || os.Args[0] == "" {
		return ""
	}
	return filepath.Base(os.Args[0])
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
	// DSN selects the backend. Format: "<scheme>://<location>[?url_prefix=…]".
	// Supported schemes:
	//   - file:// — local filesystem. SQLAlchemy/JDBC convention:
	//     "file:///relative" (3 slashes) or "file:////absolute"
	//     (4 slashes). One leading slash is stripped on parse.
	// The optional ?url_prefix= query parameter sets the public URL
	// prefix for locally served attachments (defaults to /media/).
	// Default: file:///data/media?url_prefix=/media/ (relative path).
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
			PIDFile:         cmd.String("pid-file"),
			AppName:         resolveAppName(cmd.String("app-name")),
			MaxBodySize:     int(cmd.Int("max-body-size")),
			ShutdownTimeout: int(cmd.Int("shutdown-timeout")),
		},
		Database: DatabaseConfig{
			DSN: cmd.String("database-dsn"),
		},
		Storage: StorageConfig{
			DSN: cmd.String("storage-dsn"),
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
			Name:    "app-name",
			Usage:   "Human-readable application name (used by WebAuthn dialogs, SMTP From-name, the {{ appName }} template func; defaults to the binary basename)",
			Sources: FlagSources(configSource, "APP_NAME", "server.app_name"),
		},
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
		&cli.StringFlag{
			Name:    "database-dsn",
			Value:   "sqlite:///data/app.db",
			Usage:   "Database URL (sqlite:///path or postgres://host/db)", //nolint:gosec // example URL, not a credential
			Sources: FlagSources(configSource, "DATABASE_DSN", "database.dsn"),
		},
		&cli.StringFlag{
			Name:    "storage-dsn",
			Value:   "file:///data/media?url_prefix=/media/",
			Usage:   "Storage URL for attachments (file:///relative or file:////absolute; ?url_prefix= sets the public URL prefix; empty disables Storage)",
			Sources: FlagSources(configSource, "STORAGE_DSN", "storage.dsn"),
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
	}
}
