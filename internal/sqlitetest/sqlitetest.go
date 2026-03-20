// Package sqlitetest provides a helper for opening in-memory SQLite test databases
// with consistent configuration.
package sqlitetest

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// OpenDB returns an in-memory SQLite *bun.DB for testing. It uses a single
// connection (SetMaxOpenConns(1)) to ensure all operations within a test see
// the same data without needing shared cache mode.
// The database is closed automatically when the test finishes.
func OpenDB(t *testing.T) *bun.DB {
	t.Helper()

	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?_pragma=foreign_keys(1)")
	require.NoError(t, err)
	sqldb.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqldb.Close() })

	return bun.NewDB(sqldb, sqlitedialect.New())
}
