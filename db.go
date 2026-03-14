package burrow

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// checkDBDir verifies that the parent directory of a file-based DSN exists.
// In-memory databases (":memory:" or empty DSN) are skipped.
func checkDBDir(dsn string) error {
	if dsn == "" || dsn == ":memory:" || strings.HasPrefix(dsn, "file::memory") {
		return nil
	}

	// Strip query parameters from file: URIs.
	path := dsn
	if after, ok := strings.CutPrefix(path, "file:"); ok {
		path = after
		if i := strings.IndexByte(path, '?'); i >= 0 {
			path = path[:i]
		}
	}

	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("database directory %q does not exist; create it with: mkdir -p %s", dir, dir)
	}
	return nil
}

func openDB(dsn string) (*bun.DB, error) {
	if err := checkDBDir(dsn); err != nil {
		return nil, err
	}

	dsn = withTxLock(dsn)
	dsn = withPerConnPragmas(dsn)

	sqldb, err := sql.Open(sqliteshim.ShimName, dsn)
	if err != nil {
		return nil, err
	}

	sqldb.SetMaxOpenConns(10)
	sqldb.SetMaxIdleConns(5)
	sqldb.SetConnMaxLifetime(time.Hour)

	db := bun.NewDB(sqldb, sqlitedialect.New())

	// Per-database PRAGMAs only need to run once (they persist in the DB file).
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_size_limit=27103364"); err != nil {
		return nil, fmt.Errorf("set journal size limit: %w", err)
	}

	return db, nil
}

// withPerConnPragmas appends per-connection PRAGMAs to the DSN via _pragma
// parameters. Unlike db.Exec(), _pragma parameters are applied to every new
// connection the pool creates, ensuring settings like foreign_keys=ON are
// always active.
func withPerConnPragmas(dsn string) string {
	pragmas := []string{
		"_pragma=foreign_keys(1)",
		"_pragma=synchronous(normal)",
		"_pragma=busy_timeout(5000)",
		"_pragma=temp_store(memory)",
		"_pragma=mmap_size(134217728)",
		"_pragma=cache_size(2000)",
	}

	sep := "&"
	if !strings.Contains(dsn, "?") {
		sep = "?"
	}

	return dsn + sep + strings.Join(pragmas, "&")
}

// withTxLock ensures the DSN uses IMMEDIATE transaction mode.
// This prevents transactions from failing immediately when the database is
// locked and instead waits up to busy_timeout before returning an error.
func withTxLock(dsn string) string {
	if strings.Contains(dsn, "_txlock=") {
		return dsn
	}

	switch {
	case dsn == ":memory:" || strings.HasPrefix(dsn, "file::memory"):
		return dsn
	case strings.HasPrefix(dsn, "file:"):
		if strings.Contains(dsn, "?") {
			return dsn + "&_txlock=immediate"
		}
		return dsn + "?_txlock=immediate"
	default:
		return "file:" + dsn + "?_txlock=immediate"
	}
}
