package server

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

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

// testWidget is a document type for migration/registration tests.
type testWidget struct {
	document.Base
	Name  string `json:"name" den:"index"`
	Color string `json:"color"`
}

// shutdownApp implements App + burrowapp.HasShutdown and records call order.
type shutdownApp struct { //nolint:govet // fieldalignment: readability over optimization
	name  string
	order *[]string
	err   error
}

func (a *shutdownApp) Name() string { return a.name }
func (a *shutdownApp) Shutdown(_ context.Context) error {
	*a.order = append(*a.order, a.name)
	return a.err
}

// configurableApp tracks Configure call order.
type configurableApp struct { //nolint:govet // fieldalignment: readability over optimization
	name  string
	order *[]string
}

func (a *configurableApp) Name() string { return a.name }
func (a *configurableApp) Configure(_ *burrowapp.AppConfig, _ *cli.Command) error {
	*a.order = append(*a.order, a.name+".Configure")
	return nil
}

// postConfigurableApp tracks both Configure and PostConfigure call order.
type postConfigurableApp struct { //nolint:govet // fieldalignment: readability over optimization
	name  string
	order *[]string
}

func (a *postConfigurableApp) Name() string { return a.name }
func (a *postConfigurableApp) Configure(_ *burrowapp.AppConfig, _ *cli.Command) error {
	*a.order = append(*a.order, a.name+".Configure")
	return nil
}
func (a *postConfigurableApp) PostConfigure(_ *burrowapp.AppConfig, _ *cli.Command) error {
	*a.order = append(*a.order, a.name+".PostConfigure")
	return nil
}

// postConfigErrorApp returns an error from PostConfigure.
type postConfigErrorApp struct {
	name string
}

func (a *postConfigErrorApp) Name() string                                           { return a.name }
func (a *postConfigErrorApp) Configure(_ *burrowapp.AppConfig, _ *cli.Command) error { return nil }
func (a *postConfigErrorApp) PostConfigure(_ *burrowapp.AppConfig, _ *cli.Command) error {
	return errors.New("boom")
}

// routeApp implements App + burrowapp.HasRoutes for routes tests.
type routeApp struct {
	name string
}

func (a *routeApp) Name() string { return a.name }
func (a *routeApp) Routes(r chi.Router) {
	r.Get("/from-app", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("route-registered"))
	})
}

// --- registerApp + logCapabilities tests ---

func TestRegisterApp_LogsCapabilities(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(oldLogger) })

	reg := registry.New()
	registerApp(reg, &fullApp{})

	output := buf.String()
	assert.Contains(t, output, "app registered")
	assert.Contains(t, output, "full")
	assert.Contains(t, output, "documents")
	assert.Contains(t, output, "routes")
	assert.Contains(t, output, "middleware")
	assert.Contains(t, output, "nav")
	assert.Contains(t, output, "flags")
	assert.Contains(t, output, "config")
	assert.Contains(t, output, "commands")
	assert.Contains(t, output, "migrations")
	assert.Contains(t, output, "admin")
}

func TestRegisterApp_LogsMinimalCapabilities(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(oldLogger) })

	reg := registry.New()
	registerApp(reg, &minimalApp{})

	output := buf.String()
	assert.Contains(t, output, "app registered")
	assert.Contains(t, output, "minimal")
	assert.NotContains(t, output, "migrations")
	assert.NotContains(t, output, "routes")
}

// --- configureAll tests ---

func TestConfigureAll_CallsConfigure(t *testing.T) {
	reg := registry.New()
	app1 := &trackingApp{name: "first"}
	app2 := &trackingApp{name: "second"}
	registry.Add(reg, app1)
	registry.Add(reg, app2)

	err := configureAll(reg, &burrowapp.AppConfig{Registry: reg})
	require.NoError(t, err)
	assert.True(t, app1.configured)
	assert.True(t, app2.configured)
}

func TestConfigureAll_PassesConfig(t *testing.T) {
	reg := registry.New()
	var received *burrowapp.AppConfig
	app := &trackingApp{
		name: "checker",
		configureFn: func(cfg *burrowapp.AppConfig) error {
			received = cfg
			return nil
		},
	}
	registry.Add(reg, app)

	appCfg := &burrowapp.AppConfig{Registry: reg}
	err := configureAll(reg, appCfg)
	require.NoError(t, err)
	require.NotNil(t, received)
	assert.Equal(t, reg, received.Registry)
}

func TestConfigureAll_StopsOnError(t *testing.T) {
	reg := registry.New()
	errBoom := errors.New("boom")
	app1 := &failingApp{name: "bad", err: errBoom}
	app2 := &trackingApp{name: "never"}
	registry.Add(reg, app1)
	registry.Add(reg, app2)

	err := configureAll(reg, &burrowapp.AppConfig{Registry: reg})
	require.ErrorIs(t, err, errBoom)
	assert.Contains(t, err.Error(), "bad")
	assert.False(t, app2.configured)
}

// --- configure (two-phase) tests ---

func TestConfigure_CallsConfigurableApps(t *testing.T) {
	reg := registry.New()
	app1 := &trackingApp{name: "conf1"}
	app2 := &trackingApp{name: "conf2"}
	registry.Add(reg, app1)
	registry.Add(reg, app2)
	registry.Add(reg, &minimalApp{}) // Not burrowapp.Configurable, should be skipped.

	err := configure(reg, &burrowapp.AppConfig{Registry: reg}, nil)
	require.NoError(t, err)
	assert.True(t, app1.configured)
	assert.True(t, app2.configured)
}

func TestConfigure_StopsOnError(t *testing.T) {
	reg := registry.New()
	errCfg := errors.New("config error")
	registry.Add(reg, &failingApp{name: "bad-cfg", err: errCfg})

	err := configure(reg, &burrowapp.AppConfig{Registry: reg}, nil)
	require.ErrorIs(t, err, errCfg)
	assert.Contains(t, err.Error(), "bad-cfg")
}

func TestConfigure_PostConfigureRunsAfterAllConfigure(t *testing.T) {
	var order []string
	a1 := &configurableApp{name: "alpha", order: &order}
	a2 := &postConfigurableApp{name: "beta", order: &order}
	a3 := &configurableApp{name: "gamma", order: &order}

	reg := registry.New()
	registry.Add(reg, a1)
	registry.Add(reg, a2)
	registry.Add(reg, a3)

	err := configure(reg, &burrowapp.AppConfig{Registry: reg}, nil)
	require.NoError(t, err)

	// All Configure calls must happen before any PostConfigure call.
	assert.Equal(t, []string{
		"alpha.Configure",
		"beta.Configure",
		"gamma.Configure",
		"beta.PostConfigure",
	}, order)
}

func TestConfigure_PostConfigureError(t *testing.T) {
	var order []string
	a1 := &configurableApp{name: "alpha", order: &order}

	reg := registry.New()
	registry.Add(reg, a1)
	registry.Add(reg, &postConfigErrorApp{name: "failing"})

	err := configure(reg, &burrowapp.AppConfig{Registry: reg}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "post-configure app \"failing\"")
}

func TestConfigure_SkipsNonPostConfigurable(t *testing.T) {
	var order []string
	a1 := &configurableApp{name: "alpha", order: &order}
	a2 := &configurableApp{name: "beta", order: &order}

	reg := registry.New()
	registry.Add(reg, a1)
	registry.Add(reg, a2)

	err := configure(reg, &burrowapp.AppConfig{Registry: reg}, nil)
	require.NoError(t, err)

	// Only Configure calls, no PostConfigure.
	assert.Equal(t, []string{"alpha.Configure", "beta.Configure"}, order)
}

// --- registerMiddleware tests ---

func TestRegisterMiddleware(t *testing.T) {
	reg := registry.New()

	called := false
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			next.ServeHTTP(w, r)
		})
	}
	registry.Add(reg, &trackingApp{name: "mw-app", middleware: []func(http.Handler) http.Handler{mw}})
	registry.Add(reg, &minimalApp{}) // No middleware, should be skipped.

	r := chi.NewRouter()
	registerMiddleware(reg, r)

	r.Get("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.True(t, called)
}

// --- registerRoutes tests ---

func TestRegisterRoutes(t *testing.T) {
	reg := registry.New()

	registry.Add(reg, &routeApp{name: "router"})
	registry.Add(reg, &minimalApp{}) // No routes, should be skipped.

	r := chi.NewRouter()
	registerRoutes(reg, r)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/from-app", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "route-registered", rec.Body.String())
}

// --- runMigrations tests ---

func TestRunMigrations_AppliesPendingOnce(t *testing.T) {
	ctx := t.Context()
	db := testDB(t)

	var ran int
	reg := registry.New()
	registry.Add(reg, &minimalApp{}) // no migrations — should be skipped
	registry.Add(reg, &trackingApp{
		name: "m1",
		migrations: []burrowapp.NamedMigration{{
			Version: "001_initial",
			Migration: migrate.Migration{
				Forward: func(_ context.Context, _ *den.Tx) error {
					ran++
					return nil
				},
			},
		}},
	})

	require.NoError(t, runMigrations(ctx, reg, db))
	assert.Equal(t, 1, ran)

	require.NoError(t, runMigrations(ctx, reg, db))
	assert.Equal(t, 1, ran, "second invocation must be a no-op — _den_migrations records the version")
}

func TestRunMigrations_ReturnsForwardError(t *testing.T) {
	ctx := t.Context()
	db := testDB(t)

	errBoom := errors.New("forward failed")
	reg := registry.New()
	registry.Add(reg, &trackingApp{
		name: "broken",
		migrations: []burrowapp.NamedMigration{{
			Version: "001_initial",
			Migration: migrate.Migration{
				Forward: func(_ context.Context, _ *den.Tx) error { return errBoom },
			},
		}},
	})

	err := runMigrations(ctx, reg, db)
	require.ErrorIs(t, err, errBoom)
}

// --- registerDocuments tests ---

func TestRegisterDocuments_CreatesTable(t *testing.T) {
	db := testDB(t)
	reg := registry.New()

	app := &docApp{name: "widgets", docs: []document.Document{&testWidget{}}}
	registry.Add(reg, app)

	err := registerDocuments(t.Context(), reg, db)
	require.NoError(t, err)

	// Verify table was created by inserting a document.
	w := &testWidget{Name: "gear", Color: "red"}
	err = den.Save(t.Context(), db, w)
	require.NoError(t, err)
	assert.NotEmpty(t, w.ID)
}

func TestRegisterDocuments_SkipsNonDocumentApps(t *testing.T) {
	db := testDB(t)
	reg := registry.New()

	registry.Add(reg, &docApp{name: "docapp", docs: []document.Document{&testWidget{}}})
	registry.Add(reg, &minimalApp{}) // Not burrowapp.HasDocuments, should be skipped.

	err := registerDocuments(t.Context(), reg, db)
	require.NoError(t, err)

	// Verify the document type from docapp was registered.
	w := &testWidget{Name: "test"}
	err = den.Save(t.Context(), db, w)
	require.NoError(t, err)
}

func TestRegisterDocuments_Idempotent(t *testing.T) {
	db := testDB(t)
	reg := registry.New()

	app := &docApp{name: "widgets", docs: []document.Document{&testWidget{}}}
	registry.Add(reg, app)

	// Register three times — all should succeed.
	for range 3 {
		err := registerDocuments(t.Context(), reg, db)
		require.NoError(t, err)
	}

	// Verify the table works correctly (proves no duplicate errors).
	w := &testWidget{Name: "gear", Color: "red"}
	err := den.Save(t.Context(), db, w)
	require.NoError(t, err)
}

func TestRegisterDocuments_MultipleApps(t *testing.T) {
	db := testDB(t)
	reg := registry.New()

	// testSetting is a second document type.
	type testSetting struct {
		document.Base
		Key   string `json:"key" den:"unique"`
		Value string `json:"value"`
	}

	registry.Add(reg, &docApp{name: "app_a", docs: []document.Document{&testWidget{}}})
	registry.Add(reg, &docApp{name: "app_b", docs: []document.Document{&testSetting{}}})

	err := registerDocuments(t.Context(), reg, db)
	require.NoError(t, err)

	// Verify both document types are registered.
	w := &testWidget{Name: "test"}
	err = den.Save(t.Context(), db, w)
	require.NoError(t, err)

	s := &testSetting{Key: "theme", Value: "dark"}
	err = den.Save(t.Context(), db, s)
	require.NoError(t, err)
}

func TestRegisterDocuments_WithDependencyOrder(t *testing.T) {
	db := testDB(t)
	reg := registry.New()

	// Register in dependency order.
	registry.Add(reg, &docApp{name: "base", docs: []document.Document{&testWidget{}}})
	registry.Add(reg, &struct {
		docApp
	}{docApp: docApp{name: "child", docs: nil}})

	err := registerDocuments(t.Context(), reg, db)
	require.NoError(t, err)

	// Verify widget table works.
	w := &testWidget{Name: "test"}
	err = den.Save(t.Context(), db, w)
	require.NoError(t, err)
}

// --- allFlags tests ---

func TestAllFlags(t *testing.T) {
	reg := registry.New()
	flag1 := &cli.StringFlag{Name: "auth-key"}
	flag2 := &cli.BoolFlag{Name: "debug"}
	registry.Add(reg, &trackingApp{name: "app1", flags: []cli.Flag{flag1}})
	registry.Add(reg, &trackingApp{name: "app2", flags: []cli.Flag{flag2}})
	registry.Add(reg, &minimalApp{}) // Not burrowapp.HasFlags, should be skipped.

	flags := allFlags(reg, nil)
	require.Len(t, flags, 2)
	assert.Equal(t, flag1, flags[0])
	assert.Equal(t, flag2, flags[1])
}

// --- allNavItems / allAdminNavItems tests ---

func TestAllNavItems_SortedByPosition(t *testing.T) {
	reg := registry.New()
	registry.Add(reg, &trackingApp{
		name: "app1",
		navItems: []burrowapp.NavItem{
			{Label: "Settings", Position: 30},
			{Label: "Dashboard", Position: 10},
		},
	})
	registry.Add(reg, &trackingApp{
		name: "app2",
		navItems: []burrowapp.NavItem{
			{Label: "Profile", Position: 20},
		},
	})
	// minimalApp doesn't implement burrowapp.HasNavItems - should be skipped.
	registry.Add(reg, &minimalApp{})

	items := allNavItems(reg)
	require.Len(t, items, 3)
	assert.Equal(t, "Dashboard", items[0].Label)
	assert.Equal(t, "Profile", items[1].Label)
	assert.Equal(t, "Settings", items[2].Label)
}

func TestAllNavItems_Empty(t *testing.T) {
	reg := registry.New()
	registry.Add(reg, &minimalApp{})

	items := allNavItems(reg)
	assert.Empty(t, items)
}

func TestAllAdminNavItems_SortedByPosition(t *testing.T) {
	reg := registry.New()
	registry.Add(reg, &trackingApp{
		name: "app1",
		adminNavItems: []burrowapp.NavItem{
			{Label: "Users", Position: 10},
			{Label: "Settings", Position: 30},
		},
	})
	registry.Add(reg, &trackingApp{
		name: "app2",
		adminNavItems: []burrowapp.NavItem{
			{Label: "Invites", Position: 20},
		},
	})
	registry.Add(reg, &minimalApp{})

	items := allAdminNavItems(reg)
	require.Len(t, items, 3)
	assert.Equal(t, "Users", items[0].Label)
	assert.Equal(t, "Invites", items[1].Label)
	assert.Equal(t, "Settings", items[2].Label)
}

func TestAllAdminNavItems_Empty(t *testing.T) {
	reg := registry.New()
	registry.Add(reg, &minimalApp{})

	items := allAdminNavItems(reg)
	assert.Empty(t, items)
}

// --- allCLICommands tests ---

func TestAllCLICommands(t *testing.T) {
	reg := registry.New()
	cmd1 := &cli.Command{Name: "migrate"}
	cmd2 := &cli.Command{Name: "seed"}
	registry.Add(reg, &trackingApp{name: "app1", commands: []*cli.Command{cmd1}})
	registry.Add(reg, &trackingApp{name: "app2", commands: []*cli.Command{cmd2}})
	registry.Add(reg, &minimalApp{}) // No commands, should be skipped.

	cmds := allCLICommands(reg)
	require.Len(t, cmds, 2)
	assert.Equal(t, "migrate", cmds[0].Name)
	assert.Equal(t, "seed", cmds[1].Name)
}

// --- shutdownApps tests ---

func TestShutdownApps_ReverseOrder(t *testing.T) {
	var order []string
	a1 := &shutdownApp{name: "first", order: &order}
	a2 := &shutdownApp{name: "second", order: &order}
	a3 := &shutdownApp{name: "third", order: &order}

	reg := registry.New()
	registry.Add(reg, a1)
	registry.Add(reg, a2)
	registry.Add(reg, a3)

	err := shutdownApps(context.Background(), reg)
	require.NoError(t, err)
	assert.Equal(t, []string{"third", "second", "first"}, order)
}

func TestShutdownApps_ErrorIsolation(t *testing.T) {
	var order []string
	a1 := &shutdownApp{name: "first", order: &order}
	a2 := &shutdownApp{name: "second", order: &order, err: errors.New("boom")}
	a3 := &shutdownApp{name: "third", order: &order}

	reg := registry.New()
	registry.Add(reg, a1)
	registry.Add(reg, a2)
	registry.Add(reg, a3)

	err := shutdownApps(context.Background(), reg)
	require.Error(t, err)
	// All three apps should still be called despite the error.
	assert.Equal(t, []string{"third", "second", "first"}, order)
	assert.Contains(t, err.Error(), "second")
}

func TestShutdownApps_SkipsNonImplementing(t *testing.T) {
	var order []string
	a1 := &shutdownApp{name: "with-shutdown", order: &order}
	a2 := &minimalApp{}

	reg := registry.New()
	registry.Add(reg, a1)
	registry.Add(reg, a2)

	err := shutdownApps(context.Background(), reg)
	require.NoError(t, err)
	assert.Equal(t, []string{"with-shutdown"}, order)
}
