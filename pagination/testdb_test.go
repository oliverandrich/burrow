package pagination

import (
	"path/filepath"
	"testing"

	"github.com/oliverandrich/den"
	_ "github.com/oliverandrich/den/backend/sqlite" // register sqlite:// scheme for tests
)

// testDB returns a file-backed SQLite *den.DB for integration tests in this
// package. Mirrors the testDB helper in package burrow — duplicated rather
// than imported because burrowtest imports burrow, which would cycle back
// here via the burrow.PageRequest alias.
func testDB(t *testing.T) *den.DB {
	t.Helper()
	dsn := "sqlite:///" + filepath.Join(t.TempDir(), "test.db")
	db, err := den.OpenURL(t.Context(), dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
