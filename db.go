package burrow

import (
	"context"

	"github.com/oliverandrich/den"
	_ "github.com/oliverandrich/den/backend/postgres" // register postgres:// scheme
	_ "github.com/oliverandrich/den/backend/sqlite"   // register sqlite:// scheme
	"github.com/oliverandrich/den/validate"
)

// OpenDB opens a database from a URL-style DSN with struct-tag validation
// enabled by default. Documents with `validate:"..."` tags will be
// checked before every Insert and Update; a violation returns an error
// wrapping den.ErrValidation.
//
// Supported schemes:
//   - sqlite:///path/to/db — SQLite file database
//   - postgres://user:pass@host:5432/db — PostgreSQL
//
// For the rare case where you need the pre-v0.12.0 behavior (no tag
// validation at the data layer), use OpenDBWithoutValidation.
func OpenDB(ctx context.Context, dsn string) (*den.DB, error) {
	return den.OpenURL(ctx, dsn, validate.WithValidation())
}

// OpenDBWithoutValidation opens a database with struct-tag validation
// DISABLED. This is an escape hatch for migration scenarios where an
// existing project has `validate:"..."` tags on documents that previously
// had no effect, and fixing all the data violations in one pass is not
// practical.
//
// New projects should use OpenDB. Prefer cleaning up the validation tags
// and switching back to OpenDB as soon as possible.
func OpenDBWithoutValidation(ctx context.Context, dsn string) (*den.DB, error) {
	return den.OpenURL(ctx, dsn)
}
