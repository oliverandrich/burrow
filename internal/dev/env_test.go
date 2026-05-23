package dev

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadOrGenerateEnv_GeneratesWhenMissing(t *testing.T) {
	root := t.TempDir()
	cfg := Config{ProjectRoot: root, EnvFile: ".env"}
	var log bytes.Buffer

	vars, err := loadOrGenerateEnv(cfg, &log)
	require.NoError(t, err)

	assert.Contains(t, vars, "SESSION_HASH_KEY")
	assert.Contains(t, vars, "CSRF_KEY")
	assert.Len(t, vars["SESSION_HASH_KEY"], 64, "32-byte hex = 64 chars")
	assert.Len(t, vars["CSRF_KEY"], 64)
	assert.NotEqual(t, vars["SESSION_HASH_KEY"], vars["CSRF_KEY"], "keys must be distinct")

	assert.Contains(t, log.String(), "generated .env")

	// The file persists with restrictive permissions.
	info, err := os.Stat(filepath.Join(root, ".env"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestLoadOrGenerateEnv_ReadsExistingFileVerbatim(t *testing.T) {
	root := t.TempDir()
	envPath := filepath.Join(root, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte("FOO=bar\nBAZ=\"quoted value\"\n"), 0o600))

	cfg := Config{ProjectRoot: root, EnvFile: ".env"}
	var log bytes.Buffer

	vars, err := loadOrGenerateEnv(cfg, &log)
	require.NoError(t, err)
	assert.Equal(t, "bar", vars["FOO"])
	assert.Equal(t, "quoted value", vars["BAZ"])
	assert.NotContains(t, vars, "SESSION_HASH_KEY", "must not patch foreign env files")
	assert.Empty(t, log.String(), "no notice when file already exists")
}

func TestLoadOrGenerateEnv_DisabledWhenEnvFileEmpty(t *testing.T) {
	cfg := Config{ProjectRoot: t.TempDir(), EnvFile: ""}
	var log bytes.Buffer
	vars, err := loadOrGenerateEnv(cfg, &log)
	require.NoError(t, err)
	assert.Nil(t, vars)
	assert.Empty(t, log.String())
}

func TestEnsureEnv_CreatesMissingFile(t *testing.T) {
	root := t.TempDir()
	var log bytes.Buffer
	require.NoError(t, EnsureEnv(Config{ProjectRoot: root, EnvFile: ".env"}, &log))

	info, err := os.Stat(filepath.Join(root, ".env"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	assert.Contains(t, log.String(), "generated .env")
}

func TestEnsureEnv_LeavesExistingFileUntouched(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".env"), []byte("USER_KEY=keep-me\n"), 0o600))

	var log bytes.Buffer
	require.NoError(t, EnsureEnv(Config{ProjectRoot: root, EnvFile: ".env"}, &log))

	data, err := os.ReadFile(filepath.Join(root, ".env"))
	require.NoError(t, err)
	assert.Equal(t, "USER_KEY=keep-me\n", string(data))
	assert.Empty(t, log.String(), "no notice when file already exists")
}

func TestEnsureEnv_RequiresEnvFile(t *testing.T) {
	err := EnsureEnv(Config{ProjectRoot: t.TempDir(), EnvFile: ""}, io.Discard)
	require.Error(t, err)
}

func TestMergeEnv_ShellWinsOverFile(t *testing.T) {
	base := []string{"PATH=/usr/bin", "HOME=/home/x", "SHELL=/bin/sh"}
	overrides := map[string]string{
		"HOME":   "/tmp",  // shadowed by base — base wins
		"EXTRA":  "added", // new key — applied
		"SECRET": "shh",
	}

	out := mergeEnv(base, overrides)

	assert.Contains(t, out, "PATH=/usr/bin")
	assert.Contains(t, out, "HOME=/home/x", "shell value must win over env-file override")
	assert.NotContains(t, out, "HOME=/tmp")
	assert.Contains(t, out, "SHELL=/bin/sh")
	assert.Contains(t, out, "EXTRA=added")
	assert.Contains(t, out, "SECRET=shh")
}

func TestMergeEnv_EmptyOverridesReturnsBase(t *testing.T) {
	base := []string{"A=1", "B=2"}
	assert.Equal(t, base, mergeEnv(base, nil))
	assert.Equal(t, base, mergeEnv(base, map[string]string{}))
}
