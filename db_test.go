package burrow

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenDB_SQLite(t *testing.T) {
	dsn := "sqlite:///" + filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping(t.Context()))
}

func TestOpenDB_SQLiteMemory(t *testing.T) {
	db, err := OpenDB("sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping(t.Context()))
}

func TestOpenDB_UnsupportedScheme(t *testing.T) {
	_, err := OpenDB("mysql://localhost/test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestOpenDB_EmptyDSN(t *testing.T) {
	_, err := OpenDB("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestOpenDB_NoScheme(t *testing.T) {
	_, err := OpenDB("app.db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing scheme")
}
