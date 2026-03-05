package burrow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// runCommand creates a cli.Command with CoreFlags and runs it with the given args,
// returning the parsed command for use with configureTLS.
func runCommand(t *testing.T, args ...string) *cli.Command {
	t.Helper()
	cmd := &cli.Command{
		Name:   "test",
		Flags:  CoreFlags(nil),
		Action: func(_ context.Context, _ *cli.Command) error { return nil },
	}
	err := cmd.Run(t.Context(), append([]string{"test"}, args...))
	require.NoError(t, err)
	return cmd
}

func TestConfigureTLS_Off(t *testing.T) {
	cmd := runCommand(t, "--tls-mode", "off", "--host", "localhost", "--port", "9090")
	cfg := NewConfig(cmd)

	setup, err := configureTLS(cfg)
	require.NoError(t, err)

	assert.Nil(t, setup.tlsConfig)
	assert.Equal(t, "localhost:9090", setup.addr)
	assert.Nil(t, setup.httpHandler)
	assert.Empty(t, setup.httpAddr)
}

func TestConfigureTLS_SelfSigned(t *testing.T) {
	certDir := t.TempDir()
	cmd := runCommand(t, "--tls-mode", "selfsigned", "--host", "myhost.local", "--tls-cert-dir", certDir)
	cfg := NewConfig(cmd)

	setup, err := configureTLS(cfg)
	require.NoError(t, err)

	assert.NotNil(t, setup.tlsConfig)
	assert.Len(t, setup.tlsConfig.Certificates, 1)
	assert.Nil(t, setup.httpHandler)

	// Cert files should exist on disk.
	assert.FileExists(t, filepath.Join(certDir, "selfsigned-cert.pem"))
	assert.FileExists(t, filepath.Join(certDir, "selfsigned-key.pem"))
}

func TestConfigureTLS_SelfSignedReusesExisting(t *testing.T) {
	certDir := t.TempDir()
	cmd := runCommand(t, "--tls-mode", "selfsigned", "--host", "myhost.local", "--tls-cert-dir", certDir)
	cfg := NewConfig(cmd)

	// First call generates certs.
	_, err := configureTLS(cfg)
	require.NoError(t, err)

	certFile := filepath.Join(certDir, "selfsigned-cert.pem")
	info1, err := os.Stat(certFile)
	require.NoError(t, err)

	// Second call reuses existing certs.
	_, err = configureTLS(cfg)
	require.NoError(t, err)

	info2, err := os.Stat(certFile)
	require.NoError(t, err)
	assert.Equal(t, info1.ModTime(), info2.ModTime(), "cert file should not be regenerated")
}

func TestConfigureTLS_Manual(t *testing.T) {
	// Generate a temporary cert/key pair for manual mode.
	certDir := t.TempDir()
	certFile := filepath.Join(certDir, "cert.pem")
	keyFile := filepath.Join(certDir, "key.pem")
	err := generateSelfSignedCert("manual.local", certFile, keyFile)
	require.NoError(t, err)

	cmd := runCommand(t,
		"--tls-mode", "manual",
		"--tls-cert-file", certFile,
		"--tls-key-file", keyFile,
		"--host", "manual.local",
		"--port", "8443",
	)
	cfg := NewConfig(cmd)

	setup, err := configureTLS(cfg)
	require.NoError(t, err)

	assert.NotNil(t, setup.tlsConfig)
	assert.Len(t, setup.tlsConfig.Certificates, 1)
	assert.Equal(t, "manual.local:8443", setup.addr)
	assert.Nil(t, setup.httpHandler)
}

func TestConfigureTLS_ACME(t *testing.T) {
	certDir := t.TempDir()
	cmd := runCommand(t,
		"--tls-mode", "acme",
		"--host", "example.com",
		"--tls-cert-dir", certDir,
		"--tls-email", "admin@example.com",
	)
	cfg := NewConfig(cmd)

	setup, err := configureTLS(cfg)
	require.NoError(t, err)

	assert.NotNil(t, setup.tlsConfig)
	assert.NotNil(t, setup.httpHandler)
	assert.Equal(t, ":443", setup.addr)
	assert.Equal(t, ":80", setup.httpAddr)
}

func TestConfigureTLS_AutoLocalhost(t *testing.T) {
	cmd := runCommand(t, "--host", "localhost")
	cfg := NewConfig(cmd)

	setup, err := configureTLS(cfg)
	require.NoError(t, err)

	assert.Nil(t, setup.tlsConfig, "auto+localhost should resolve to off")
	assert.Equal(t, "localhost:8080", setup.addr)
}

func TestConfigureTLS_AutoRemote(t *testing.T) {
	cmd := runCommand(t, "--host", "example.com")
	cfg := NewConfig(cmd)

	setup, err := configureTLS(cfg)
	require.NoError(t, err)

	assert.NotNil(t, setup.tlsConfig, "auto+remote should resolve to acme")
	assert.Equal(t, ":443", setup.addr)
	assert.NotNil(t, setup.httpHandler)
}

func TestConfigureTLS_UnknownMode(t *testing.T) {
	cmd := runCommand(t, "--tls-mode", "bogus")
	cfg := NewConfig(cmd)

	_, err := configureTLS(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown TLS mode")
}
