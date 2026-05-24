package selfupdate

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstall_ReplacesTargetAtomic(t *testing.T) {
	target := filepath.Join(t.TempDir(), "myapp")
	require.NoError(t, os.WriteFile(target, []byte("old-binary"), 0o755))

	newContent := []byte("new-binary-bytes")
	sum := sha256.Sum256(newContent)

	err := install(bytes.NewReader(newContent), target, sum[:])
	require.NoError(t, err)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, newContent, got)
}

func TestInstall_RejectsChecksumMismatch(t *testing.T) {
	target := filepath.Join(t.TempDir(), "myapp")
	require.NoError(t, os.WriteFile(target, []byte("old-binary"), 0o755))

	newContent := []byte("new-binary-bytes")
	wrongSum := sha256.Sum256([]byte("something else"))

	err := install(bytes.NewReader(newContent), target, wrongSum[:])
	require.Error(t, err)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, []byte("old-binary"), got, "target must be unchanged on checksum failure")
}
