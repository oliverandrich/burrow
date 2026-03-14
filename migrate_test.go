package burrow

import (
	"bytes"
	"database/sql"
	"io/fs"
	"log/slog"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func testDB(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?_pragma=foreign_keys(1)")
	require.NoError(t, err)
	t.Cleanup(func() { sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	return db
}

func TestRunAppMigrationsCreatesTrackingTable(t *testing.T) {
	db := testDB(t)
	migrations := fstest.MapFS{
		"001_create_items.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE items (id INTEGER PRIMARY KEY);"),
		},
	}

	err := RunAppMigrations(t.Context(), db, "myapp", migrations)
	require.NoError(t, err)

	// Verify tracking table exists with the migration recorded.
	var count int
	err = db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ? AND name = ?",
		"myapp", "001_create_items.up.sql").Scan(t.Context(), &count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRunAppMigrationsExecutesSQL(t *testing.T) {
	db := testDB(t)
	migrations := fstest.MapFS{
		"001_create_items.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT);"),
		},
	}

	err := RunAppMigrations(t.Context(), db, "testapp", migrations)
	require.NoError(t, err)

	// Verify the table was created.
	_, err = db.NewRaw("INSERT INTO items (name) VALUES (?)", "test").Exec(t.Context())
	require.NoError(t, err)
}

func TestRunAppMigrationsSkipsApplied(t *testing.T) {
	db := testDB(t)
	migrations := fstest.MapFS{
		"001_create_items.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE items (id INTEGER PRIMARY KEY);"),
		},
	}

	// Run twice - second run should not error (idempotent).
	err := RunAppMigrations(t.Context(), db, "testapp", migrations)
	require.NoError(t, err)
	err = RunAppMigrations(t.Context(), db, "testapp", migrations)
	require.NoError(t, err)
}

func TestRunAppMigrationsRunsInOrder(t *testing.T) {
	db := testDB(t)
	migrations := fstest.MapFS{
		"001_create_items.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE items (id INTEGER PRIMARY KEY);"),
		},
		"002_add_name.up.sql": &fstest.MapFile{
			Data: []byte("ALTER TABLE items ADD COLUMN name TEXT;"),
		},
	}

	err := RunAppMigrations(t.Context(), db, "testapp", migrations)
	require.NoError(t, err)

	// Verify both applied.
	var count int
	err = db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ?", "testapp").
		Scan(t.Context(), &count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify the column exists (proves order was correct).
	_, err = db.NewRaw("INSERT INTO items (name) VALUES (?)", "test").Exec(t.Context())
	require.NoError(t, err)
}

func TestRunAppMigrationsIgnoresDownFiles(t *testing.T) {
	db := testDB(t)
	migrations := fstest.MapFS{
		"001_create_items.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE items (id INTEGER PRIMARY KEY);"),
		},
		"001_create_items.down.sql": &fstest.MapFile{
			Data: []byte("DROP TABLE items;"),
		},
	}

	err := RunAppMigrations(t.Context(), db, "testapp", migrations)
	require.NoError(t, err)

	// Table should exist (down migration was not executed).
	_, err = db.NewRaw("SELECT COUNT(*) FROM items").Exec(t.Context())
	require.NoError(t, err)
}

func TestRunAppMigrationsNamespacesPerApp(t *testing.T) {
	db := testDB(t)
	migrationsA := fstest.MapFS{
		"001_create_items.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE items_a (id INTEGER PRIMARY KEY);"),
		},
	}
	migrationsB := fstest.MapFS{
		"001_create_items.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE items_b (id INTEGER PRIMARY KEY);"),
		},
	}

	err := RunAppMigrations(t.Context(), db, "app_a", migrationsA)
	require.NoError(t, err)
	err = RunAppMigrations(t.Context(), db, "app_b", migrationsB)
	require.NoError(t, err)

	// Both apps have their own migration tracked.
	var countA, countB int
	err = db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ?", "app_a").
		Scan(t.Context(), &countA)
	require.NoError(t, err)
	err = db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ?", "app_b").
		Scan(t.Context(), &countB)
	require.NoError(t, err)
	assert.Equal(t, 1, countA)
	assert.Equal(t, 1, countB)
}

func TestRunAppMigrationsRollsBackOnFailure(t *testing.T) {
	db := testDB(t)

	// First migration succeeds, second fails mid-way.
	// The second migration creates a table then runs invalid SQL.
	// With transactions, the partial table creation should be rolled back.
	migrations := fstest.MapFS{
		"001_create_items.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE items (id INTEGER PRIMARY KEY);"),
		},
		"002_bad_migration.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE orphan (id INTEGER PRIMARY KEY);\nTHIS IS NOT SQL;"),
		},
	}

	err := RunAppMigrations(t.Context(), db, "testapp", migrations)
	require.Error(t, err)

	// First migration should have committed (it was in its own transaction).
	var count int
	err = db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ? AND name = ?",
		"testapp", "001_create_items.up.sql").Scan(t.Context(), &count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "first migration should be recorded")

	// Second migration should NOT be recorded.
	err = db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ? AND name = ?",
		"testapp", "002_bad_migration.up.sql").Scan(t.Context(), &count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "failed migration should not be recorded")

	// The orphan table from the failed migration should not exist (rolled back).
	_, err = db.NewRaw("SELECT COUNT(*) FROM orphan").Exec(t.Context())
	assert.Error(t, err, "orphan table should not exist after rollback")
}

func TestRunAppMigrationsReturnsErrorOnBadSQL(t *testing.T) {
	db := testDB(t)
	migrations := fstest.MapFS{
		"001_bad.up.sql": &fstest.MapFile{
			Data: []byte("THIS IS NOT SQL;"),
		},
	}

	err := RunAppMigrations(t.Context(), db, "badapp", migrations)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "badapp")
	assert.Contains(t, err.Error(), "001_bad.up.sql")
}

func TestRunAppMigrationsEmptyFS(t *testing.T) {
	db := testDB(t)
	migrations := fstest.MapFS{}

	err := RunAppMigrations(t.Context(), db, "empty", migrations)
	require.NoError(t, err)
}

func TestRegistryRunMigrations(t *testing.T) {
	db := testDB(t)
	reg := NewRegistry()

	migFS := fstest.MapFS{
		"001_create_things.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE things (id INTEGER PRIMARY KEY);"),
		},
	}

	reg.Add(&migratableApp{name: "migapp", fs: migFS})
	reg.Add(&minimalApp{}) // Not Migratable, should be skipped.

	err := reg.RunMigrations(t.Context(), db)
	require.NoError(t, err)

	// Verify migration was tracked under app name.
	var count int
	err = db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ?", "migapp").
		Scan(t.Context(), &count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRunAppMigrationsLogsAppliedMigrations(t *testing.T) {
	db := testDB(t)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))) })

	migrations := fstest.MapFS{
		"001_create_items.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE items (id INTEGER PRIMARY KEY);"),
		},
		"002_add_name.up.sql": &fstest.MapFile{
			Data: []byte("ALTER TABLE items ADD COLUMN name TEXT;"),
		},
	}

	err := RunAppMigrations(t.Context(), db, "testapp", migrations)
	require.NoError(t, err)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "migration applied")
	assert.Contains(t, logOutput, "001_create_items.up.sql")
	assert.Contains(t, logOutput, "002_add_name.up.sql")
	assert.Contains(t, logOutput, "testapp")
}

func TestRunAppMigrationsDoesNotLogSkippedMigrations(t *testing.T) {
	db := testDB(t)

	migrations := fstest.MapFS{
		"001_create_items.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE items (id INTEGER PRIMARY KEY);"),
		},
	}

	// Apply first.
	err := RunAppMigrations(t.Context(), db, "testapp", migrations)
	require.NoError(t, err)

	// Run again with logging captured.
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))) })

	err = RunAppMigrations(t.Context(), db, "testapp", migrations)
	require.NoError(t, err)

	assert.NotContains(t, buf.String(), "migration applied")
}

// migratableApp is a test helper implementing App + Migratable.
type migratableApp struct {
	fs   fs.FS
	name string
}

func (a *migratableApp) Name() string                { return a.name }
func (a *migratableApp) Register(_ *AppConfig) error { return nil }
func (a *migratableApp) MigrationFS() fs.FS          { return a.fs }

// depApp is a test helper implementing App + Migratable + HasDependencies.
type depApp struct {
	fs   fs.FS
	name string
	deps []string
}

func (a *depApp) Name() string                { return a.name }
func (a *depApp) Register(_ *AppConfig) error { return nil }
func (a *depApp) MigrationFS() fs.FS          { return a.fs }
func (a *depApp) Dependencies() []string      { return a.deps }

func TestMigrationDependencyOrder(t *testing.T) {
	db := testDB(t)
	reg := NewRegistry()

	// App "base" creates the base table.
	baseFS := fstest.MapFS{
		"001_base.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE base (id INTEGER PRIMARY KEY, name TEXT);"),
		},
	}
	// App "child" depends on "base" and references its table via a foreign key.
	childFS := fstest.MapFS{
		"001_child.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE child (id INTEGER PRIMARY KEY, base_id INTEGER NOT NULL REFERENCES base(id));"),
		},
	}
	// App "grandchild" depends on "child" and references its table.
	grandchildFS := fstest.MapFS{
		"001_grandchild.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE grandchild (id INTEGER PRIMARY KEY, child_id INTEGER NOT NULL REFERENCES child(id));"),
		},
	}

	// Register in dependency order (enforced by Registry.Add).
	reg.Add(&migratableApp{name: "base", fs: baseFS})
	reg.Add(&depApp{name: "child", fs: childFS, deps: []string{"base"}})
	reg.Add(&depApp{name: "grandchild", fs: grandchildFS, deps: []string{"child"}})

	err := reg.RunMigrations(t.Context(), db)
	require.NoError(t, err)

	// Verify all three apps' migrations were recorded.
	var count int
	err = db.NewRaw("SELECT COUNT(*) FROM _migrations").Scan(t.Context(), &count)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Verify the foreign key chain works by inserting data.
	_, err = db.NewRaw("INSERT INTO base (id, name) VALUES (1, 'root')").Exec(t.Context())
	require.NoError(t, err)
	_, err = db.NewRaw("INSERT INTO child (id, base_id) VALUES (1, 1)").Exec(t.Context())
	require.NoError(t, err)
	_, err = db.NewRaw("INSERT INTO grandchild (id, child_id) VALUES (1, 1)").Exec(t.Context())
	require.NoError(t, err)
}

func TestMigrationPartialFailureRecovery(t *testing.T) {
	db := testDB(t)
	reg := NewRegistry()

	// App "good" has a valid migration.
	goodFS := fstest.MapFS{
		"001_create_users.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT);"),
		},
	}
	// App "bad" has two migrations: first succeeds, second fails.
	badFS := fstest.MapFS{
		"001_create_orders.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER);"),
		},
		"002_broken.up.sql": &fstest.MapFile{
			Data: []byte("ALTER TABLE nonexistent ADD COLUMN broken TEXT;"),
		},
	}

	reg.Add(&migratableApp{name: "good", fs: goodFS})
	reg.Add(&migratableApp{name: "bad", fs: badFS})

	err := reg.RunMigrations(t.Context(), db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "002_broken.up.sql")

	// "good" app's migration should have been applied successfully.
	var goodCount int
	err = db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ?", "good").
		Scan(t.Context(), &goodCount)
	require.NoError(t, err)
	assert.Equal(t, 1, goodCount, "good app migration should be recorded")

	// "bad" app's first migration should have been applied.
	var badCount int
	err = db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ? AND name = ?",
		"bad", "001_create_orders.up.sql").Scan(t.Context(), &badCount)
	require.NoError(t, err)
	assert.Equal(t, 1, badCount, "bad app's first migration should be recorded")

	// "bad" app's second (failed) migration should NOT be recorded.
	err = db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ? AND name = ?",
		"bad", "002_broken.up.sql").Scan(t.Context(), &badCount)
	require.NoError(t, err)
	assert.Equal(t, 0, badCount, "bad app's failed migration should not be recorded")

	// Verify the users table from "good" is intact.
	_, err = db.NewRaw("INSERT INTO users (email) VALUES ('test@example.com')").Exec(t.Context())
	require.NoError(t, err)

	// Verify the orders table from "bad"'s successful first migration is intact.
	_, err = db.NewRaw("INSERT INTO orders (user_id) VALUES (1)").Exec(t.Context())
	require.NoError(t, err)
}

func TestMigrationIdempotency(t *testing.T) {
	db := testDB(t)
	reg := NewRegistry()

	migFS := fstest.MapFS{
		"001_create_widgets.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT);"),
		},
		"002_add_color.up.sql": &fstest.MapFile{
			Data: []byte("ALTER TABLE widgets ADD COLUMN color TEXT;"),
		},
	}

	reg.Add(&migratableApp{name: "widgets", fs: migFS})

	// Run migrations three times. All should succeed.
	for range 3 {
		err := reg.RunMigrations(t.Context(), db)
		require.NoError(t, err)
	}

	// Verify migrations are recorded exactly once each.
	var count int
	err := db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ?", "widgets").
		Scan(t.Context(), &count)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "each migration should be recorded exactly once")

	// Verify the table works correctly (proves no duplicate ALTER TABLE etc.).
	_, err = db.NewRaw("INSERT INTO widgets (name, color) VALUES ('gear', 'red')").Exec(t.Context())
	require.NoError(t, err)
}

func TestMigrationIdempotencyAfterPartialFailure(t *testing.T) {
	db := testDB(t)

	// First run: migration 001 succeeds, 002 fails.
	migrations := fstest.MapFS{
		"001_create_settings.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE settings (id INTEGER PRIMARY KEY, key TEXT);"),
		},
		"002_invalid.up.sql": &fstest.MapFile{
			Data: []byte("THIS IS INVALID SQL;"),
		},
	}

	err := RunAppMigrations(t.Context(), db, "app", migrations)
	require.Error(t, err)

	// Fix the broken migration and re-run.
	migrationsFixed := fstest.MapFS{
		"001_create_settings.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE settings (id INTEGER PRIMARY KEY, key TEXT);"),
		},
		"002_invalid.up.sql": &fstest.MapFile{
			Data: []byte("ALTER TABLE settings ADD COLUMN value TEXT;"),
		},
	}

	err = RunAppMigrations(t.Context(), db, "app", migrationsFixed)
	require.NoError(t, err)

	// Verify both migrations are now recorded.
	var count int
	err = db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ?", "app").
		Scan(t.Context(), &count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify the table has both columns.
	_, err = db.NewRaw("INSERT INTO settings (key, value) VALUES ('theme', 'dark')").Exec(t.Context())
	require.NoError(t, err)
}
