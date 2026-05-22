package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	burrowapp "github.com/oliverandrich/burrow/app"
	"github.com/oliverandrich/burrow/registry"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"
	"github.com/oliverandrich/den/migrate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestNewServer(t *testing.T) {
	app1 := &minimalApp{}
	app2 := &fullApp{}

	s := New(app1, app2)

	require.NotNil(t, s)
	apps := registry.Apps(s.Registry())
	require.Len(t, apps, 2)
	assert.Equal(t, "minimal", apps[0].Name())
	assert.Equal(t, "full", apps[1].Name())
}

func TestServerFlags(t *testing.T) {
	appWithFlags := &trackingApp{
		name:  "flaggy",
		flags: []cli.Flag{&cli.StringFlag{Name: "flaggy-key"}},
	}
	s := New(appWithFlags)

	flags := s.Flags(nil)

	names := make(map[string]bool)
	for _, f := range flags {
		names[f.Names()[0]] = true
	}

	// Core flags present.
	assert.True(t, names["host"])
	assert.True(t, names["port"])
	assert.True(t, names["database-dsn"])
	assert.True(t, names["storage-dsn"])

	// App flags present.
	assert.True(t, names["flaggy-key"])
}

// testThing is a document type used in server bootstrap tests.
type testThing struct {
	document.Base
	Label string `json:"label"`
}

func TestServerBootstrap(t *testing.T) {
	app := &docApp{name: "things", docs: []document.Document{&testThing{}}}

	s := New(app)
	db := testDB(t)

	err := s.bootstrap(t.Context(), db, nil)
	require.NoError(t, err)

	// Document type was registered — verify by inserting.
	thing := &testThing{Label: "test"}
	err = den.Save(t.Context(), db, thing)
	require.NoError(t, err)
	assert.NotEmpty(t, thing.ID)
}

func TestServerBootstrapCreatesAppConfig(t *testing.T) {
	s := New(&minimalApp{})
	db := testDB(t)

	cfg := &burrowapp.Config{Server: burrowapp.ServerConfig{Host: "testhost", Port: 9090}}
	err := s.bootstrap(t.Context(), db, cfg)
	require.NoError(t, err)

	require.NotNil(t, s.appCfg)
	assert.Equal(t, db, s.appCfg.DB)
	assert.Equal(t, "testhost", s.appCfg.Config.Server.Host)
}

func TestServerBootstrapDoesNotRunMigrations(t *testing.T) {
	var ran bool
	app := &trackingApp{
		name: "migrator",
		migrations: []burrowapp.NamedMigration{{
			Version: "001",
			Migration: migrate.Migration{
				Forward: func(_ context.Context, _ *den.Tx) error { ran = true; return nil },
			},
		}},
	}

	s := New(app)
	db := testDB(t)

	err := s.bootstrap(t.Context(), db, nil)
	require.NoError(t, err)

	assert.False(t, ran, "bootstrap only registers documents; migrations run later via runMigrations after Configure")
}

func TestSetLayout(t *testing.T) {
	s := New(&minimalApp{})

	s.SetLayout("app/layout")
	assert.Equal(t, "app/layout", s.layout)
}

func TestLayoutMiddleware(t *testing.T) {
	r := chi.NewRouter()
	r.Use(layoutMiddleware("test/layout"))

	var got string
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		got = burrowapp.Layout(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test/layout", got, "layout should be set in context")
}

func TestNavItemsMiddleware(t *testing.T) {
	items := []burrowapp.NavItem{
		{Label: "Home", URL: "/", Position: 1},
		{Label: "About", URL: "/about", Position: 2},
	}

	r := chi.NewRouter()
	r.Use(navItemsMiddleware(items))

	var gotItems []burrowapp.NavItem
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		gotItems = burrowapp.NavItems(r.Context())
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

func TestServerRunAction(t *testing.T) {
	app := &trackingApp{name: "testapp"}
	s := New(app)

	// Build a CLI command that exercises the full Run path. Cancel the
	// context shortly after Run starts so the database opens, apps configure,
	// and then the serve loop exits cleanly.
	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	cmd := &cli.Command{
		Name:  "test",
		Flags: s.Flags(nil),
		Action: func(_ context.Context, cmd *cli.Command) error {
			return s.Run(ctx, cmd)
		},
	}

	err := cmd.Run(t.Context(), []string{"test", "--database-dsn", "sqlite://:memory:", "--port", "0"})

	// The server should start and stop cleanly on cancelled context.
	require.NoError(t, err)
	assert.True(t, app.configured)
}

// TestServerCLICommandsConfigureBeforeAction pins the contract that subcommands
// returned by Server.CLICommands() run inside the framework's boot lifecycle —
// i.e. Configure() runs on every burrowapp.Configurable app before any subcommand Action
// fires. Without the wrap (raw allCLICommands), Action runs against
// uninitialised apps (a.repo == nil etc.).
func TestServerCLICommandsConfigureBeforeAction(t *testing.T) {
	var actionRan, configuredAtAction bool

	app := &trackingApp{name: "testapp"}
	app.commands = []*cli.Command{
		{
			Name: "myop",
			Action: func(_ context.Context, _ *cli.Command) error {
				configuredAtAction = app.configured
				actionRan = true
				return nil
			},
		},
	}

	s := New(app)

	cmd := &cli.Command{
		Name:     "test",
		Flags:    s.Flags(nil),
		Commands: s.CLICommands(),
	}

	err := cmd.Run(t.Context(), []string{"test", "--database-dsn", testDSN(t), "myop"})
	require.NoError(t, err)

	assert.True(t, actionRan, "subcommand Action must have fired")
	assert.True(t, configuredAtAction, "Configure must have run before the subcommand Action")
}
