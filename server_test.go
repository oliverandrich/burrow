package burrow

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestNewServer(t *testing.T) {
	app1 := &minimalApp{}
	app2 := &fullApp{}

	s := NewServer(app1, app2)

	require.NotNil(t, s)
	apps := s.Registry().Apps()
	require.Len(t, apps, 2)
	assert.Equal(t, "minimal", apps[0].Name())
	assert.Equal(t, "full", apps[1].Name())
}

func TestServerFlags(t *testing.T) {
	appWithFlags := &trackingApp{
		name:  "flaggy",
		flags: []cli.Flag{&cli.StringFlag{Name: "flaggy-key"}},
	}
	s := NewServer(appWithFlags)

	flags := s.Flags(nil)

	names := make(map[string]bool)
	for _, f := range flags {
		names[f.Names()[0]] = true
	}

	// Core flags present.
	assert.True(t, names["host"])
	assert.True(t, names["port"])
	assert.True(t, names["database-dsn"])

	// App flags present.
	assert.True(t, names["flaggy-key"])
}

func TestServerBootstrap(t *testing.T) {
	migFS := fstest.MapFS{
		"001_create_things.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE things (id INTEGER PRIMARY KEY);"),
		},
	}
	app := &migratableApp{name: "mig", fs: migFS}
	tracker := &trackingApp{name: "tracker"}

	s := NewServer(app, tracker)
	db := testDB(t)

	err := s.bootstrap(t.Context(), db, nil)
	require.NoError(t, err)

	// Migration was applied.
	var count int
	err = db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ?", "mig").
		Scan(t.Context(), &count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// App was registered via Bootstrap.
	assert.True(t, tracker.registered)
}

func TestServerBootstrapSetsConfig(t *testing.T) {
	var receivedCfg *AppConfig
	app := &trackingApp{
		name: "checker",
		registerFn: func(cfg *AppConfig) error {
			receivedCfg = cfg
			return nil
		},
	}

	s := NewServer(app)
	db := testDB(t)

	cfg := &Config{Server: ServerConfig{Host: "testhost", Port: 9090}}
	err := s.bootstrap(t.Context(), db, cfg)
	require.NoError(t, err)

	require.NotNil(t, receivedCfg)
	assert.Equal(t, db, receivedCfg.DB)
	assert.Equal(t, "testhost", receivedCfg.Config.Server.Host)
}

func TestServerBootstrapCallsSeed(t *testing.T) {
	app := &trackingApp{name: "seedable"}

	s := NewServer(app)
	db := testDB(t)

	err := s.bootstrap(t.Context(), db, nil)
	require.NoError(t, err)

	assert.True(t, app.seeded, "bootstrap should call Seed on Seedable apps")
}

func TestServerBootstrapSeedError(t *testing.T) {
	seedErr := errors.New("seed failed")
	app := &failingApp{name: "bad-seed", failOn: "seed", err: seedErr}

	s := NewServer(app)
	db := testDB(t)

	err := s.bootstrap(t.Context(), db, nil)
	require.ErrorIs(t, err, seedErr)
	assert.Contains(t, err.Error(), "seed")
}

func TestSetLayout(t *testing.T) {
	s := NewServer(&minimalApp{})

	layout := LayoutFunc(func(_ http.ResponseWriter, _ *http.Request, _ int, content template.HTML, _ map[string]any) error {
		return nil
	})

	s.SetLayout(layout)
	assert.NotNil(t, s.layout)
}

func TestLayoutMiddleware(t *testing.T) {
	layout := LayoutFunc(func(_ http.ResponseWriter, _ *http.Request, _ int, _ template.HTML, _ map[string]any) error {
		return nil
	})

	r := chi.NewRouter()
	r.Use(layoutMiddleware(layout))

	var got LayoutFunc
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		got = Layout(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotNil(t, got, "layout should be set in context")
}

func TestNavItemsMiddleware(t *testing.T) {
	items := []NavItem{
		{Label: "Home", URL: "/", Position: 1},
		{Label: "About", URL: "/about", Position: 2},
	}

	r := chi.NewRouter()
	r.Use(navItemsMiddleware(items))

	var gotItems []NavItem
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		gotItems = NavItems(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, gotItems, 2)
	assert.Equal(t, "Home", gotItems[0].Label)
	assert.Equal(t, "About", gotItems[1].Label)
}

func TestOpenDBMissingDirectory(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "nonexistent", "subdir", "app.db")
	_, err := openDB(dsn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
	assert.Contains(t, err.Error(), "mkdir -p")
	assert.NotContains(t, err.Error(), "out of memory")
}

func TestCheckDBDirSkipsMemory(t *testing.T) {
	assert.NoError(t, checkDBDir(":memory:"))
	assert.NoError(t, checkDBDir(""))
	assert.NoError(t, checkDBDir("file::memory:?cache=shared"))
}

func TestCheckDBDirExistingDir(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "app.db")
	assert.NoError(t, checkDBDir(dsn))
}

func TestCheckDBDirFileURI(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "nonexistent", "app.db") + "?mode=rwc"
	err := checkDBDir(dsn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
}

func TestWithTxLock(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "plain file path",
			dsn:  "app.db",
			want: "file:app.db?_txlock=immediate",
		},
		{
			name: "file URI without params",
			dsn:  "file:app.db",
			want: "file:app.db?_txlock=immediate",
		},
		{
			name: "file URI with existing params",
			dsn:  "file:app.db?mode=rwc",
			want: "file:app.db?mode=rwc&_txlock=immediate",
		},
		{
			name: "already has txlock",
			dsn:  "file:app.db?_txlock=deferred",
			want: "file:app.db?_txlock=deferred",
		},
		{
			name: "memory database",
			dsn:  ":memory:",
			want: ":memory:",
		},
		{
			name: "file memory with cache",
			dsn:  "file::memory:?cache=shared",
			want: "file::memory:?cache=shared",
		},
		{
			name: "absolute path",
			dsn:  "/var/data/app.db",
			want: "file:/var/data/app.db?_txlock=immediate",
		},
		{
			name: "file URI absolute path",
			dsn:  "file:/var/data/app.db",
			want: "file:/var/data/app.db?_txlock=immediate",
		},
		{
			name: "txlock in middle of params",
			dsn:  "file:app.db?_txlock=immediate&mode=rwc",
			want: "file:app.db?_txlock=immediate&mode=rwc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withTxLock(tt.dsn)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestServerRunAction(t *testing.T) {
	app := &trackingApp{name: "testapp"}
	s := NewServer(app)

	// Build a CLI command that exercises the full Run path but
	// cancels the context immediately so the server doesn't block.
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately.

	cmd := &cli.Command{
		Name:  "test",
		Flags: s.Flags(nil),
		Action: func(_ context.Context, cmd *cli.Command) error {
			return s.Run(ctx, cmd)
		},
	}

	err := cmd.Run(t.Context(), []string{"test", "--database-dsn", ":memory:"})

	// The server should start and stop cleanly on cancelled context.
	require.NoError(t, err)
	assert.True(t, app.registered)
}
