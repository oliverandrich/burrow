package burrow

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFreshDB_MigrationOnEmptyDatabase(t *testing.T) {
	db := TestDB(t)
	migrations := fstest.MapFS{
		"001_create_users.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);"),
		},
		"002_add_email.up.sql": &fstest.MapFile{
			Data: []byte("ALTER TABLE users ADD COLUMN email TEXT;"),
		},
	}

	err := RunAppMigrations(t.Context(), db, "fresh_test", migrations)
	require.NoError(t, err)

	// Verify both migrations were applied.
	var count int
	err = db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ?", "fresh_test").
		Scan(t.Context(), &count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify the table exists and accepts inserts with all columns.
	_, err = db.NewRaw("INSERT INTO users (name, email) VALUES (?, ?)", "test", "test@example.com").
		Exec(t.Context())
	require.NoError(t, err)
}

func TestFreshDB_EmptyTableReturnsEmptyResults(t *testing.T) {
	db := TestDB(t)
	migrations := fstest.MapFS{
		"001_create_items.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT NOT NULL);"),
		},
	}

	err := RunAppMigrations(t.Context(), db, "items_app", migrations)
	require.NoError(t, err)

	// Query the empty table — should return zero rows, not an error.
	var names []string
	err = db.NewRaw("SELECT name FROM items").Scan(t.Context(), &names)
	require.NoError(t, err)
	assert.Empty(t, names)
}

func TestFreshDB_ServerBootstrapWithMultipleApps(t *testing.T) {
	migA := fstest.MapFS{
		"001_create_a.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE table_a (id INTEGER PRIMARY KEY, val TEXT);"),
		},
	}
	migB := fstest.MapFS{
		"001_create_b.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE table_b (id INTEGER PRIMARY KEY, ref_a INTEGER REFERENCES table_a(id));"),
		},
	}

	appA := &migratableApp{name: "app_a", fs: migA}
	appB := &depApp{name: "app_b", fs: migB, deps: []string{"app_a"}}

	srv := NewServer(appA, appB)
	db := TestDB(t)

	err := srv.bootstrap(t.Context(), db, nil)
	require.NoError(t, err)

	// Both tables should exist and accept inserts respecting the foreign key.
	_, err = db.NewRaw("INSERT INTO table_a (id, val) VALUES (1, 'hello')").Exec(t.Context())
	require.NoError(t, err)
	_, err = db.NewRaw("INSERT INTO table_b (id, ref_a) VALUES (1, 1)").Exec(t.Context())
	require.NoError(t, err)
}

func TestFreshDB_ServerBootstrapWithNoMigrations(t *testing.T) {
	app := &minimalApp{}
	srv := NewServer(app)
	db := TestDB(t)

	err := srv.bootstrap(t.Context(), db, nil)
	require.NoError(t, err)

	// Verify the app was registered successfully.
	apps := srv.Registry().Apps()
	require.Len(t, apps, 1)
	assert.Equal(t, "minimal", apps[0].Name())
}

func TestFreshDB_EmptyListEndpointReturnsOK(t *testing.T) {
	migFS := fstest.MapFS{
		"001_create_items.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT);"),
		},
	}

	db := TestDB(t)
	err := RunAppMigrations(t.Context(), db, "list_app", migFS)
	require.NoError(t, err)

	// Build a handler that counts items from the empty table.
	r := chi.NewRouter()
	r.Get("/items", Handle(func(w http.ResponseWriter, r *http.Request) error {
		var count int
		if err := db.NewRaw("SELECT COUNT(*) FROM items").Scan(r.Context(), &count); err != nil {
			return NewHTTPError(http.StatusInternalServerError, "query failed")
		}
		return JSON(w, http.StatusOK, map[string]int{"count": count})
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"count":0`)
}

func TestFreshDB_MigrationSystemHandlesRepeatedBootstrap(t *testing.T) {
	migFS := fstest.MapFS{
		"001_create_widgets.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE widgets (id INTEGER PRIMARY KEY);"),
		},
	}

	app := &migratableApp{name: "widgets", fs: migFS}
	srv := NewServer(app)
	db := TestDB(t)

	// Bootstrap twice — second run should be idempotent.
	err := srv.bootstrap(t.Context(), db, nil)
	require.NoError(t, err)

	err = srv.bootstrap(t.Context(), db, nil)
	require.NoError(t, err)

	// Verify migration recorded exactly once.
	var count int
	err = db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ?", "widgets").
		Scan(t.Context(), &count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestFreshDB_BootstrapAndHandleRequestsCleanly(t *testing.T) {
	migFS := fstest.MapFS{
		"001_create_things.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE things (id INTEGER PRIMARY KEY, label TEXT NOT NULL);"),
		},
	}

	app := &migratableApp{name: "things", fs: migFS}
	srv := NewServer(app)
	db := TestDB(t)

	err := srv.bootstrap(t.Context(), db, nil)
	require.NoError(t, err)

	// Simulate a request to a fresh (empty) table.
	r := chi.NewRouter()
	r.Get("/things", Handle(func(w http.ResponseWriter, r *http.Request) error {
		var count int
		if err := db.NewRaw("SELECT COUNT(*) FROM things").Scan(r.Context(), &count); err != nil {
			return NewHTTPError(http.StatusInternalServerError, "query failed")
		}
		return JSON(w, http.StatusOK, map[string]int{"count": count})
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/things", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"count":0`)
}
