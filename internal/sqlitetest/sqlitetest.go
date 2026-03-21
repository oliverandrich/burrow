// Package sqlitetest provides a helper for opening in-memory SQLite test databases
// with consistent configuration.
package sqlitetest

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// OpenDB returns a file-backed SQLite *bun.DB for testing. It creates the
// database in t.TempDir() so it is automatically cleaned up. A file-backed
// database avoids the data-loss hazard of :memory: databases whose content
// disappears when the connection pool closes and reopens a connection.
// The database is closed automatically when the test finishes.
func OpenDB(t *testing.T) *bun.DB {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "test.db") + "?_pragma=foreign_keys(1)"
	sqldb, err := sql.Open(sqliteshim.ShimName, dsn)
	require.NoError(t, err)
	sqldb.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqldb.Close() })

	return bun.NewDB(sqldb, sqlitedialect.New())
}
