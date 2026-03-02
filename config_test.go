package burrow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestCoreFlags(t *testing.T) {
	flags := CoreFlags(nil)

	names := make(map[string]bool)
	for _, f := range flags {
		names[f.Names()[0]] = true
	}

	expected := []string{
		"host", "port", "base-url", "max-body-size", "shutdown-timeout",
		"database-dsn",
		"tls-mode", "tls-cert-dir", "tls-email", "tls-cert-file", "tls-key-file",
	}
	for _, name := range expected {
		assert.True(t, names[name], "missing flag: %s", name)
	}
}

func testCommand(flags []cli.Flag) *cli.Command {
	return &cli.Command{
		Name:   "test",
		Flags:  flags,
		Action: func(_ context.Context, _ *cli.Command) error { return nil },
	}
}

func TestCoreDefaultValues(t *testing.T) {
	cmd := testCommand(CoreFlags(nil))

	err := cmd.Run(t.Context(), []string{"test"})
	require.NoError(t, err)

	cfg := NewConfig(cmd)
	assert.Equal(t, "localhost", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Empty(t, cfg.Server.BaseURL)
	assert.Equal(t, 1, cfg.Server.MaxBodySize)
	assert.Equal(t, 10, cfg.Server.ShutdownTimeout)
	assert.Equal(t, "./data/app.db", cfg.Database.DSN)
	assert.Equal(t, "auto", cfg.TLS.Mode)
	assert.Equal(t, "./data/certs", cfg.TLS.CertDir)
}

func TestCoreFlagOverrides(t *testing.T) {
	cmd := testCommand(CoreFlags(nil))

	err := cmd.Run(t.Context(), []string{
		"test",
		"--host", "0.0.0.0",
		"--port", "3000",
		"--database-dsn", "/tmp/test.db",
	})
	require.NoError(t, err)

	cfg := NewConfig(cmd)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, "/tmp/test.db", cfg.Database.DSN)
}

func TestShutdownTimeoutOverride(t *testing.T) {
	cmd := testCommand(CoreFlags(nil))

	err := cmd.Run(t.Context(), []string{"test", "--shutdown-timeout", "30"})
	require.NoError(t, err)

	cfg := NewConfig(cmd)
	assert.Equal(t, 30, cfg.Server.ShutdownTimeout)
}

func TestBuildBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		tlsMode  string
		expected string
		port     int
	}{
		{"localhost http", "localhost", "off", "http://localhost:8080", 8080},
		{"localhost default port", "localhost", "off", "http://localhost", 80},
		{"https default port", "example.com", "manual", "https://example.com", 443},
		{"https custom port", "example.com", "manual", "https://example.com:8443", 8443},
		{"acme always 443", "example.com", "acme", "https://example.com", 8080},
		{"auto local", "localhost", "auto", "http://localhost:8080", 8080},
		{"auto remote", "example.com", "auto", "https://example.com:8080", 8080},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Server: ServerConfig{Host: tt.host, Port: tt.port},
				TLS:    TLSConfig{Mode: tt.tlsMode},
			}
			result := cfg.ResolveBaseURL()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsLocalhost(t *testing.T) {
	assert.True(t, IsLocalhost("localhost"))
	assert.True(t, IsLocalhost("127.0.0.1"))
	assert.True(t, IsLocalhost("::1"))
	assert.True(t, IsLocalhost("app.localhost"))
	assert.True(t, IsLocalhost(""))
	assert.False(t, IsLocalhost("example.com"))
}

func TestConfigInAppConfig(t *testing.T) {
	cfg := &AppConfig{
		Config: &Config{
			Server: ServerConfig{Host: "myhost"},
		},
	}
	assert.Equal(t, "myhost", cfg.Config.Server.Host)
}
