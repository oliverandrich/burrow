package burrow

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/uptrace/bun"
)

const migrationsTable = `CREATE TABLE IF NOT EXISTS _migrations (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	app TEXT NOT NULL,
	name TEXT NOT NULL,
	applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(app, name)
)`

// RunAppMigrations runs all unapplied .up.sql migrations from the given FS
// for the named app. Migrations are tracked in the _migrations table,
// namespaced by app name.
func RunAppMigrations(ctx context.Context, db *bun.DB, appName string, migrations fs.FS) error {
	if _, err := db.ExecContext(ctx, migrationsTable); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrations, ".")
	if err != nil {
		return fmt.Errorf("read migrations for app %q: %w", appName, err)
	}

	// Collect and sort .up.sql files.
	var upFiles []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		upFiles = append(upFiles, entry.Name())
	}
	sort.Strings(upFiles)

	for _, name := range upFiles {
		var count int
		err := db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ? AND name = ?",
			appName, name).Scan(ctx, &count)
		if err != nil {
			return fmt.Errorf("check migration %q for app %q: %w", name, appName, err)
		}
		if count > 0 {
			continue
		}

		content, err := fs.ReadFile(migrations, name)
		if err != nil {
			return fmt.Errorf("read migration %q for app %q: %w", name, appName, err)
		}

		if err := runMigrationInTx(ctx, db, appName, name, string(content)); err != nil {
			return err
		}

		slog.Info("migration applied", "app", appName, "migration", name)
	}

	return nil
}

// runMigrationInTx executes a single migration and records it in _migrations,
// all within a transaction. If either step fails, the entire transaction is
// rolled back to prevent partial state.
func runMigrationInTx(ctx context.Context, db *bun.DB, appName, name, sql string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction for migration %q of app %q: %w", name, appName, err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, sql); err != nil {
		return fmt.Errorf("execute migration %q for app %q: %w", name, appName, err)
	}

	if _, err := tx.ExecContext(ctx,
		"INSERT INTO _migrations (app, name) VALUES (?, ?)",
		appName, name,
	); err != nil {
		return fmt.Errorf("record migration %q for app %q: %w", name, appName, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %q for app %q: %w", name, appName, err)
	}
	return nil
}

// RunMigrations runs migrations for all Migratable apps in registration order.
func (r *Registry) RunMigrations(ctx context.Context, db *bun.DB) error {
	for _, app := range r.apps {
		m, ok := app.(Migratable)
		if !ok {
			continue
		}
		if err := RunAppMigrations(ctx, db, app.Name(), m.MigrationFS()); err != nil {
			return err
		}
	}
	return nil
}
