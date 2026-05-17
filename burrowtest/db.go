// Package burrowtest provides test helpers for code that uses the burrow
// framework. It mirrors the standard-library convention of putting test
// utilities in a sibling sub-package (httptest, fstest, iotest), keeping
// the production package's import graph free of test-only dependencies.
//
// Importing burrowtest blank-imports the SQLite Den backend, since [DB]
// opens a SQLite database. Production binaries that do not import this
// package therefore do not link the SQLite engine.
package burrowtest

import (
	"path/filepath"
	"testing"

	"github.com/oliverandrich/den"
	_ "github.com/oliverandrich/den/backend/sqlite" // register sqlite:// scheme
)

// DB returns a file-backed SQLite database wrapped in a [den.DB] for
// testing. Struct-tag validation is always-on at the Den layer. The
// database is created in [testing.T.TempDir] and closed automatically
// when the test finishes.
func DB(t *testing.T) *den.DB {
	t.Helper()
	db, err := den.OpenURL(t.Context(), TempDSN(t))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TempDSN returns a file-backed SQLite DSN under [testing.T.TempDir],
// for tests that need to construct their own DB connection (e.g. passing
// --database-dsn to a cli.Command). The directory is cleaned up when the
// test finishes.
func TempDSN(t *testing.T) string {
	t.Helper()
	return "sqlite:///" + filepath.Join(t.TempDir(), "test.db")
}
