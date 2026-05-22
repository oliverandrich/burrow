package server

import (
	"path/filepath"
	"testing"

	"github.com/oliverandrich/den"
	_ "github.com/oliverandrich/den/backend/sqlite" // register sqlite:// scheme for internal tests
)

// testDB returns a file-backed SQLite database wrapped in a [den.DB] for
// burrow's own internal tests. Mirrors burrowtest.DB but lives in
// package burrow to avoid the circular import that would arise if the
// internal tests depended on the burrowtest sub-package.
func testDB(t *testing.T) *den.DB {
	t.Helper()
	db, err := den.OpenURL(t.Context(), testDSN(t))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// testDSN mirrors [burrowtest.TempDSN]; package burrow can't import
// burrowtest (which itself imports burrow) without a cycle.
func testDSN(t *testing.T) string {
	t.Helper()
	return "sqlite:///" + filepath.Join(t.TempDir(), "test.db")
}
