package burrow

import (
	"database/sql"
	"io/fs"
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
	sqldb, err := sql.Open(sqliteshim.ShimName, ":memory:")
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

// migratableApp is a test helper implementing App + Migratable.
type migratableApp struct {
	fs   fs.FS
	name string
}

func (a *migratableApp) Name() string                { return a.name }
func (a *migratableApp) Register(_ *AppConfig) error { return nil }
func (a *migratableApp) MigrationFS() fs.FS          { return a.fs }
