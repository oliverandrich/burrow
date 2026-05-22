package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	burrowapp "github.com/oliverandrich/burrow/app"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/storage"
	_ "github.com/oliverandrich/den/storage/file" // register file:// scheme
)

// OpenDB opens a database from a URL-style DSN. Struct-tag validation
// is always-on at the Den layer — documents with `validate:"..."` tags
// are checked before every Save; a violation returns an error wrapping
// den.ErrValidation.
//
// Supported schemes:
//   - sqlite:///path/to/db — SQLite file database
//   - postgres://user:pass@host:5432/db — PostgreSQL
//
// The matching Den backend package must be blank-imported by the
// consuming binary's main package — Burrow does not pull either backend
// in by default, so production binaries only link the engine they
// actually use:
//
//	import (
//	    _ "github.com/oliverandrich/den/backend/sqlite"   // for sqlite:// DSNs
//	    _ "github.com/oliverandrich/den/backend/postgres" // for postgres:// DSNs
//	)
//
// Additional den.Options (for example den.WithStorage) are forwarded so
// callers can layer capabilities on top.
func OpenDB(ctx context.Context, dsn string, opts ...den.Option) (*den.DB, error) {
	db, err := den.OpenURL(ctx, dsn, opts...)
	return db, wrapUnregisteredBackend(dsn, err)
}

// wrapUnregisteredBackend appends a Burrow-specific hint to Den's
// ErrUnsupportedScheme error pointing at the missing blank-import.
// Returns err unchanged for nil and for any other error.
func wrapUnregisteredBackend(dsn string, err error) error {
	if !errors.Is(err, den.ErrUnsupportedScheme) {
		return err
	}
	scheme, _, ok := strings.Cut(dsn, "://")
	if !ok {
		return err
	}
	return fmt.Errorf("%w — add `_ \"github.com/oliverandrich/den/backend/%s\"` to your main.go", err, strings.ToLower(scheme))
}

// openStorage constructs a den.Storage from a Burrow burrowapp.StorageConfig.
// Returns (nil, nil) when cfg.DSN is empty — callers treat that as
// "Storage disabled" and skip den.WithStorage.
//
// Delegates to storage.OpenURL; backends register themselves via
// side-effect imports (the file:// scheme is registered at the top of
// this file). Schemes like s3:// become available once the
// corresponding backend sub-package is imported.
func openStorage(cfg burrowapp.StorageConfig) (den.Storage, error) {
	if cfg.DSN == "" {
		return nil, nil
	}
	s, err := storage.OpenURL(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("storage-dsn %q: %w", cfg.DSN, err)
	}
	return s, nil
}
