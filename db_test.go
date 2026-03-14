package burrow

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenDBMissingDirectory(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "nonexistent", "subdir", "app.db")
	_, err := openDB(dsn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
	assert.Contains(t, err.Error(), "mkdir -p")
	assert.NotContains(t, err.Error(), "out of memory")
}

func TestCheckDBDirSkipsMemory(t *testing.T) {
	assert.NoError(t, checkDBDir(":memory:"))
	assert.NoError(t, checkDBDir(""))
	assert.NoError(t, checkDBDir("file::memory:?cache=shared"))
}

func TestCheckDBDirExistingDir(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "app.db")
	assert.NoError(t, checkDBDir(dsn))
}

func TestCheckDBDirFileURI(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "nonexistent", "app.db") + "?mode=rwc"
	err := checkDBDir(dsn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
}

func TestWithTxLock(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "plain file path",
			dsn:  "app.db",
			want: "file:app.db?_txlock=immediate",
		},
		{
			name: "file URI without params",
			dsn:  "file:app.db",
			want: "file:app.db?_txlock=immediate",
		},
		{
			name: "file URI with existing params",
			dsn:  "file:app.db?mode=rwc",
			want: "file:app.db?mode=rwc&_txlock=immediate",
		},
		{
			name: "already has txlock",
			dsn:  "file:app.db?_txlock=deferred",
			want: "file:app.db?_txlock=deferred",
		},
		{
			name: "memory database",
			dsn:  ":memory:",
			want: ":memory:",
		},
		{
			name: "file memory with cache",
			dsn:  "file::memory:?cache=shared",
			want: "file::memory:?cache=shared",
		},
		{
			name: "absolute path",
			dsn:  "/var/data/app.db",
			want: "file:/var/data/app.db?_txlock=immediate",
		},
		{
			name: "file URI absolute path",
			dsn:  "file:/var/data/app.db",
			want: "file:/var/data/app.db?_txlock=immediate",
		},
		{
			name: "txlock in middle of params",
			dsn:  "file:app.db?_txlock=immediate&mode=rwc",
			want: "file:app.db?_txlock=immediate&mode=rwc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withTxLock(tt.dsn)
			assert.Equal(t, tt.want, got)
		})
	}
}
