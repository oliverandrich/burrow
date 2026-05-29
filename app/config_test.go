package app

import (
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
		"host", "port", "base-url", "pid-file", "max-body-size", "shutdown-timeout",
		"client-ip-mode", "client-ip-header", "client-ip-trusted-proxies", "client-ip-trusted-cidrs",
		"database-dsn",
		"storage-dsn",
		"tls-mode", "tls-cert-dir", "tls-email", "tls-cert-file", "tls-key-file",
	}
	for _, name := range expected {
		assert.True(t, names[name], "missing flag: %s", name)
	}
}

func TestCoreDefaultValues(t *testing.T) {
	cmd := runCommand(t)
	cfg := NewConfig(cmd)
	assert.Equal(t, "localhost", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Empty(t, cfg.Server.BaseURL)
	assert.Empty(t, cfg.Server.PIDFile)
	assert.Equal(t, 1, cfg.Server.MaxBodySize)
	assert.Equal(t, 10, cfg.Server.ShutdownTimeout)
	assert.Equal(t, "sqlite:///data/app.db", cfg.Database.DSN)
	assert.Equal(t, "file:///data/media?url_prefix=/media/", cfg.Storage.DSN)
	assert.Equal(t, "auto", cfg.TLS.Mode)
	assert.Equal(t, "./data/certs", cfg.TLS.CertDir)
}

func TestCoreFlagOverrides(t *testing.T) {
	cmd := runCommand(t,
		"--host", "0.0.0.0",
		"--port", "3000",
		"--database-dsn", "/tmp/test.db",
	)
	cfg := NewConfig(cmd)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, "/tmp/test.db", cfg.Database.DSN)
}

func TestShutdownTimeoutOverride(t *testing.T) {
	cmd := runCommand(t, "--shutdown-timeout", "30")
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

func TestValidateTLS_ACMERejectsExplicitPort(t *testing.T) {
	cmd := runCommand(t, "--tls-mode", "acme", "--host", "example.com", "--port", "9090")
	cfg := NewConfig(cmd)

	err := cfg.ValidateTLS(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--port")
}

func TestValidateTLS_ACMEWithoutExplicitPort(t *testing.T) {
	cmd := runCommand(t, "--tls-mode", "acme", "--host", "example.com")
	cfg := NewConfig(cmd)

	err := cfg.ValidateTLS(cmd)
	require.NoError(t, err)
}

func TestValidateTLS_ManualMissingCertFile(t *testing.T) {
	cmd := runCommand(t, "--tls-mode", "manual", "--tls-key-file", "/tmp/key.pem")
	cfg := NewConfig(cmd)

	err := cfg.ValidateTLS(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--tls-cert-file")
}

func TestValidateTLS_ManualMissingKeyFile(t *testing.T) {
	cmd := runCommand(t, "--tls-mode", "manual", "--tls-cert-file", "/tmp/cert.pem")
	cfg := NewConfig(cmd)

	err := cfg.ValidateTLS(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--tls-key-file")
}

func TestValidateTLS_AutoRemoteRejectsExplicitPort(t *testing.T) {
	cmd := runCommand(t, "--host", "example.com", "--port", "9090")
	cfg := NewConfig(cmd)

	err := cfg.ValidateTLS(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--port")
}

func TestValidateTLS_OffIsAlwaysValid(t *testing.T) {
	cmd := runCommand(t, "--tls-mode", "off", "--port", "9090")
	cfg := NewConfig(cmd)

	err := cfg.ValidateTLS(cmd)
	require.NoError(t, err)
}

func TestValidateTLS_UnknownMode(t *testing.T) {
	cmd := runCommand(t, "--tls-mode", "bogus")
	cfg := NewConfig(cmd)

	err := cfg.ValidateTLS(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown TLS mode")
}

func TestClientIP_DefaultMode(t *testing.T) {
	cmd := runCommand(t)
	cfg := NewConfig(cmd)
	assert.Equal(t, "remote-addr", cfg.Server.ClientIP.Mode)
	assert.Empty(t, cfg.Server.ClientIP.Header)
	assert.Zero(t, cfg.Server.ClientIP.TrustedProxies)
	assert.Empty(t, cfg.Server.ClientIP.TrustedCIDRs)

	require.NoError(t, cfg.ValidateClientIP(cmd))
}

func TestClientIP_HeaderMode(t *testing.T) {
	cmd := runCommand(t, "--client-ip-mode", "header", "--client-ip-header", "X-Real-IP")
	cfg := NewConfig(cmd)
	assert.Equal(t, "header", cfg.Server.ClientIP.Mode)
	assert.Equal(t, "X-Real-IP", cfg.Server.ClientIP.Header)

	require.NoError(t, cfg.ValidateClientIP(cmd))
}

func TestValidateClientIP_HeaderModeMissingHeader(t *testing.T) {
	cmd := runCommand(t, "--client-ip-mode", "header")
	cfg := NewConfig(cmd)

	err := cfg.ValidateClientIP(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--client-ip-header")
}

func TestValidateClientIP_XFFTrustedProxiesMode(t *testing.T) {
	cmd := runCommand(t, "--client-ip-mode", "xff-trusted-proxies", "--client-ip-trusted-proxies", "2")
	cfg := NewConfig(cmd)
	assert.Equal(t, "xff-trusted-proxies", cfg.Server.ClientIP.Mode)
	assert.Equal(t, 2, cfg.Server.ClientIP.TrustedProxies)

	require.NoError(t, cfg.ValidateClientIP(cmd))
}

func TestValidateClientIP_XFFTrustedProxiesMissingCount(t *testing.T) {
	cmd := runCommand(t, "--client-ip-mode", "xff-trusted-proxies")
	cfg := NewConfig(cmd)

	err := cfg.ValidateClientIP(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--client-ip-trusted-proxies")
}

func TestValidateClientIP_XFFTrustedCIDRsMode(t *testing.T) {
	cmd := runCommand(t, "--client-ip-mode", "xff-trusted-cidrs", "--client-ip-trusted-cidrs", "13.32.0.0/15,52.46.0.0/18")
	cfg := NewConfig(cmd)
	assert.Equal(t, "xff-trusted-cidrs", cfg.Server.ClientIP.Mode)
	assert.Equal(t, []string{"13.32.0.0/15", "52.46.0.0/18"}, cfg.Server.ClientIP.TrustedCIDRs)

	require.NoError(t, cfg.ValidateClientIP(cmd))
}

func TestValidateClientIP_XFFTrustedCIDRsMissingList(t *testing.T) {
	cmd := runCommand(t, "--client-ip-mode", "xff-trusted-cidrs")
	cfg := NewConfig(cmd)

	err := cfg.ValidateClientIP(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--client-ip-trusted-cidrs")
}

func TestValidateClientIP_UnknownMode(t *testing.T) {
	cmd := runCommand(t, "--client-ip-mode", "bogus")
	cfg := NewConfig(cmd)

	err := cfg.ValidateClientIP(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown client-ip mode")
}

func TestValidateClientIP_HeaderSetWithWrongMode(t *testing.T) {
	// Companion flag set without the matching mode is a configuration
	// error — fail fast instead of silently ignoring the flag.
	cmd := runCommand(t, "--client-ip-mode", "remote-addr", "--client-ip-header", "X-Real-IP")
	cfg := NewConfig(cmd)

	err := cfg.ValidateClientIP(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--client-ip-header")
}

func TestValidateClientIP_HeaderModeRejectsTrustedProxies(t *testing.T) {
	// Cross-mode companion: --client-ip-mode=header with --client-ip-trusted-proxies
	// is meaningless — chi's ClientIPFromHeader ignores XFF. Fail at boot.
	cmd := runCommand(t,
		"--client-ip-mode", "header",
		"--client-ip-header", "X-Real-IP",
		"--client-ip-trusted-proxies", "2",
	)
	cfg := NewConfig(cmd)

	err := cfg.ValidateClientIP(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--client-ip-trusted-proxies")
}

func TestValidateClientIP_XFFTrustedProxiesRejectsCIDRs(t *testing.T) {
	cmd := runCommand(t,
		"--client-ip-mode", "xff-trusted-proxies",
		"--client-ip-trusted-proxies", "2",
		"--client-ip-trusted-cidrs", "10.0.0.0/8",
	)
	cfg := NewConfig(cmd)

	err := cfg.ValidateClientIP(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--client-ip-trusted-cidrs")
}

func TestValidateClientIP_XFFTrustedCIDRsRejectsHeader(t *testing.T) {
	cmd := runCommand(t,
		"--client-ip-mode", "xff-trusted-cidrs",
		"--client-ip-trusted-cidrs", "10.0.0.0/8",
		"--client-ip-header", "X-Real-IP",
	)
	cfg := NewConfig(cmd)

	err := cfg.ValidateClientIP(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--client-ip-header")
}

func TestFlagSourcesWithNilConfigSource(t *testing.T) {
	chain := FlagSources(nil, "MY_VAR", "my.key")
	// Should not panic; chain is created with only the env var source.
	assert.NotNil(t, chain)
}

func TestFlagSourcesWithConfigSource(t *testing.T) {
	called := false
	configSource := func(key string) cli.ValueSource {
		called = true
		assert.Equal(t, "app.setting", key)
		return cli.EnvVar("FALLBACK") // dummy source for testing
	}

	chain := FlagSources(configSource, "MY_VAR", "app.setting")
	assert.NotNil(t, chain)
	assert.True(t, called, "configSource should be called with the TOML key")
}

func TestConfigInAppConfig(t *testing.T) {
	cfg := &AppConfig{
		Config: &Config{
			Server: ServerConfig{Host: "myhost"},
		},
	}
	assert.Equal(t, "myhost", cfg.Config.Server.Host)
}

func TestResolveAppName_ExplicitFlagWins(t *testing.T) {
	assert.Equal(t, "My App", resolveAppName("My App"))
}

func TestResolveAppName_FallsBackToBinaryBasename(t *testing.T) {
	got := resolveAppName("")
	assert.NotEmpty(t, got, "fallback should pick up the binary basename from os.Args[0]")
	assert.NotContains(t, got, "/", "result should be a basename, not a full path")
}
