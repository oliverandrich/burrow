package burrow

import (
	"context"
	"fmt"

	"github.com/oliverandrich/den"
	_ "github.com/oliverandrich/den/backend/postgres" // register postgres:// scheme
	_ "github.com/oliverandrich/den/backend/sqlite"   // register sqlite:// scheme
	"github.com/oliverandrich/den/storage"
	_ "github.com/oliverandrich/den/storage/file" // register file:// scheme
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
// Additional den.Options (for example den.WithStorage) are appended
// after validation so callers can layer capabilities on top.
//
// For the rare case where you need the pre-v0.12.0 behavior (no tag
// validation at the data layer), use OpenDBWithoutValidation.
func OpenDB(ctx context.Context, dsn string, opts ...den.Option) (*den.DB, error) {
	return den.OpenURL(ctx, dsn, append([]den.Option{validate.WithValidation()}, opts...)...)
}

// OpenDBWithoutValidation opens a database with struct-tag validation
// DISABLED. This is an escape hatch for migration scenarios where an
// existing project has `validate:"..."` tags on documents that previously
// had no effect, and fixing all the data violations in one pass is not
// practical.
//
// New projects should use OpenDB. Prefer cleaning up the validation tags
// and switching back to OpenDB as soon as possible.
func OpenDBWithoutValidation(ctx context.Context, dsn string, opts ...den.Option) (*den.DB, error) {
	return den.OpenURL(ctx, dsn, opts...)
}

// openStorage constructs a den.Storage from a Burrow StorageConfig.
// Returns (nil, nil) when cfg.DSN is empty — callers treat that as
// "Storage disabled" and skip den.WithStorage.
//
// Delegates to storage.OpenURL; backends register themselves via
// side-effect imports (the file:// scheme is registered at the top of
// this file). Schemes like s3:// become available once the
// corresponding backend sub-package is imported.
func openStorage(cfg StorageConfig) (den.Storage, error) {
	if cfg.DSN == "" {
		return nil, nil
	}
	s, err := storage.OpenURL(cfg.DSN, cfg.URLPrefix)
	if err != nil {
		return nil, fmt.Errorf("storage-dsn %q: %w", cfg.DSN, err)
	}
	return s, nil
}
