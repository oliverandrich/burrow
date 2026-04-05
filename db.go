package burrow

import (
	"github.com/oliverandrich/den"
	_ "github.com/oliverandrich/den/backend/postgres" // register postgres:// scheme
	_ "github.com/oliverandrich/den/backend/sqlite"   // register sqlite:// scheme
)

// OpenDB opens a database from a URL-style DSN.
// Supported schemes:
//   - sqlite:///path/to/db — SQLite file database
//   - postgres://user:pass@host:5432/db — PostgreSQL
func OpenDB(dsn string) (*den.DB, error) {
	return den.OpenURL(dsn)
}
