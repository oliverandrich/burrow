package burrow

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// minimalApp implements only the required App interface.
type minimalApp struct{}

func (a *minimalApp) Name() string                { return "minimal" }
func (a *minimalApp) Register(_ *AppConfig) error { return nil }

// fullApp implements App + all optional interfaces.
type fullApp struct {
	registered bool
}

func (a *fullApp) Name() string                                  { return "full" }
func (a *fullApp) Register(_ *AppConfig) error                   { a.registered = true; return nil }
func (a *fullApp) MigrationFS() fs.FS                            { return nil }
func (a *fullApp) Middleware() []func(http.Handler) http.Handler { return nil }
func (a *fullApp) NavItems() []NavItem                           { return nil }
func (a *fullApp) Flags() []cli.Flag                             { return nil }
func (a *fullApp) Configure(_ *cli.Command) error                { return nil }
func (a *fullApp) CLICommands() []*cli.Command                   { return nil }
func (a *fullApp) Seed(_ context.Context) error                  { return nil }
func (a *fullApp) Routes(_ chi.Router)                           {}
func (a *fullApp) AdminRoutes(_ chi.Router)                      {}
func (a *fullApp) AdminNavItems() []NavItem                      { return nil }

// trackingApp records calls and provides test data for lifecycle methods.
type trackingApp struct {
	registerFn    func(cfg *AppConfig) error
	name          string
	navItems      []NavItem
	adminNavItems []NavItem
	middleware    []func(http.Handler) http.Handler
	flags         []cli.Flag
	commands      []*cli.Command
	registered    bool
	configured    bool
	seeded        bool
}

func (a *trackingApp) Name() string { return a.name }
func (a *trackingApp) Register(cfg *AppConfig) error {
	a.registered = true
	if a.registerFn != nil {
		return a.registerFn(cfg)
	}
	return nil
}
func (a *trackingApp) NavItems() []NavItem                           { return a.navItems }
func (a *trackingApp) Middleware() []func(http.Handler) http.Handler { return a.middleware }
func (a *trackingApp) Flags() []cli.Flag                             { return a.flags }
func (a *trackingApp) Configure(_ *cli.Command) error                { a.configured = true; return nil }
func (a *trackingApp) CLICommands() []*cli.Command                   { return a.commands }
func (a *trackingApp) Seed(_ context.Context) error                  { a.seeded = true; return nil }
func (a *trackingApp) AdminRoutes(_ chi.Router)                      {}
func (a *trackingApp) AdminNavItems() []NavItem                      { return a.adminNavItems }

// failingApp returns errors from Register, Configure, or Seed.
type failingApp struct {
	err     error
	name    string
	failOn  string
	reached bool
}

func (a *failingApp) Name() string { return a.name }
func (a *failingApp) Register(_ *AppConfig) error {
	if a.failOn == "register" {
		return a.err
	}
	return nil
}
func (a *failingApp) Flags() []cli.Flag { return nil }
func (a *failingApp) Configure(_ *cli.Command) error {
	if a.failOn == "configure" {
		return a.err
	}
	return nil
}
func (a *failingApp) Seed(_ context.Context) error {
	a.reached = true
	if a.failOn == "seed" {
		return a.err
	}
	return nil
}

// Compile-time interface assertions.
var (
	_ App             = (*minimalApp)(nil)
	_ App             = (*fullApp)(nil)
	_ Migratable      = (*fullApp)(nil)
	_ HasMiddleware   = (*fullApp)(nil)
	_ HasNavItems     = (*fullApp)(nil)
	_ Configurable    = (*fullApp)(nil)
	_ HasCLICommands  = (*fullApp)(nil)
	_ Seedable        = (*fullApp)(nil)
	_ HasRoutes       = (*fullApp)(nil)
	_ HasAdmin        = (*fullApp)(nil)
	_ HasDependencies = (*dependentApp)(nil)
)

func TestMinimalAppSatisfiesOnlyApp(t *testing.T) {
	var app App = &minimalApp{}
	assert.Equal(t, "minimal", app.Name())

	_, isMigratable := app.(Migratable)
	_, hasMiddleware := app.(HasMiddleware)
	_, hasNavItems := app.(HasNavItems)
	_, isConfigurable := app.(Configurable)
	_, hasCLI := app.(HasCLICommands)
	_, isSeedable := app.(Seedable)
	_, hasRoutes := app.(HasRoutes)
	_, hasAdmin := app.(HasAdmin)

	assert.False(t, isMigratable)
	assert.False(t, hasMiddleware)
	assert.False(t, hasNavItems)
	assert.False(t, isConfigurable)
	assert.False(t, hasCLI)
	assert.False(t, isSeedable)
	assert.False(t, hasRoutes)
	assert.False(t, hasAdmin)
}

func TestFullAppSatisfiesAllInterfaces(t *testing.T) {
	var app App = &fullApp{}

	_, isMigratable := app.(Migratable)
	_, hasMiddleware := app.(HasMiddleware)
	_, hasNavItems := app.(HasNavItems)
	_, isConfigurable := app.(Configurable)
	_, hasCLI := app.(HasCLICommands)
	_, isSeedable := app.(Seedable)
	_, hasRoutes := app.(HasRoutes)
	_, hasAdmin := app.(HasAdmin)

	assert.True(t, isMigratable)
	assert.True(t, hasMiddleware)
	assert.True(t, hasNavItems)
	assert.True(t, isConfigurable)
	assert.True(t, hasCLI)
	assert.True(t, isSeedable)
	assert.True(t, hasRoutes)
	assert.True(t, hasAdmin)
}

func TestNavItemFields(t *testing.T) {
	item := NavItem{
		Label:    "Dashboard",
		URL:      "/dashboard",
		Icon:     "bi bi-speedometer2",
		Position: 10,
		AuthOnly: true,
	}

	assert.Equal(t, "Dashboard", item.Label)
	assert.Equal(t, "/dashboard", item.URL)
	assert.Equal(t, "bi bi-speedometer2", item.Icon)
	assert.Equal(t, 10, item.Position)
	assert.True(t, item.AuthOnly)
}

func TestRegistryAddAndGet(t *testing.T) {
	reg := NewRegistry()
	app := &minimalApp{}

	reg.Add(app)

	got, ok := reg.Get("minimal")
	require.True(t, ok)
	assert.Equal(t, app, got)
}

func TestRegistryGetMissing(t *testing.T) {
	reg := NewRegistry()

	_, ok := reg.Get("nonexistent")
	assert.False(t, ok)
}

func TestRegistryAppsPreservesOrder(t *testing.T) {
	reg := NewRegistry()
	app1 := &minimalApp{}
	app2 := &fullApp{}

	reg.Add(app1)
	reg.Add(app2)

	apps := reg.Apps()
	require.Len(t, apps, 2)
	assert.Equal(t, "minimal", apps[0].Name())
	assert.Equal(t, "full", apps[1].Name())
}

func TestRegistryAddDuplicatePanics(t *testing.T) {
	reg := NewRegistry()
	reg.Add(&minimalApp{})

	assert.PanicsWithValue(t,
		`burrow: duplicate app name "minimal"`,
		func() { reg.Add(&minimalApp{}) },
	)
}

// dependentApp implements App + HasDependencies.
type dependentApp struct {
	deps []string
}

func (a *dependentApp) Name() string                { return "dependent" }
func (a *dependentApp) Register(_ *AppConfig) error { return nil }
func (a *dependentApp) Dependencies() []string      { return a.deps }

func TestRegistryAddPanicsOnMissingDependency(t *testing.T) {
	reg := NewRegistry()

	assert.PanicsWithValue(t,
		`burrow: app "dependent" requires "session" to be registered first`,
		func() {
			reg.Add(&dependentApp{deps: []string{"session"}})
		},
	)
}

func TestRegistryAddSucceedsWhenDependencySatisfied(t *testing.T) {
	reg := NewRegistry()
	reg.Add(&minimalApp{}) // name = "minimal"

	assert.NotPanics(t, func() {
		reg.Add(&dependentApp{deps: []string{"minimal"}})
	})

	_, ok := reg.Get("dependent")
	assert.True(t, ok)
}

func TestRegistryAddLogsCapabilities(t *testing.T) {
	// Capture slog output.
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(oldLogger) })

	reg := NewRegistry()
	reg.Add(&fullApp{})

	output := buf.String()
	assert.Contains(t, output, "app registered")
	assert.Contains(t, output, "full")
	assert.Contains(t, output, "migrations")
	assert.Contains(t, output, "routes")
	assert.Contains(t, output, "middleware")
	assert.Contains(t, output, "nav")
	assert.Contains(t, output, "config")
	assert.Contains(t, output, "commands")
	assert.Contains(t, output, "seed")
	assert.Contains(t, output, "admin")
}

func TestRegistryAddLogsMinimalCapabilities(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(oldLogger) })

	reg := NewRegistry()
	reg.Add(&minimalApp{})

	output := buf.String()
	assert.Contains(t, output, "app registered")
	assert.Contains(t, output, "minimal")
	// Minimal app has no capabilities — shouldn't list any.
	assert.NotContains(t, output, "migrations")
	assert.NotContains(t, output, "routes")
}

func TestRegistryBootstrapCallsRegister(t *testing.T) {
	reg := NewRegistry()
	app1 := &trackingApp{name: "first"}
	app2 := &trackingApp{name: "second"}
	reg.Add(app1)
	reg.Add(app2)

	err := reg.Bootstrap(nil)
	require.NoError(t, err)
	assert.True(t, app1.registered)
	assert.True(t, app2.registered)
}

func TestRegistryBootstrapPassesConfig(t *testing.T) {
	reg := NewRegistry()
	var received *AppConfig
	app := &trackingApp{
		name: "checker",
		registerFn: func(cfg *AppConfig) error {
			received = cfg
			return nil
		},
	}
	reg.Add(app)

	err := reg.Bootstrap(nil)
	require.NoError(t, err)
	require.NotNil(t, received)
	assert.Equal(t, reg, received.Registry)
}

func TestRegistryBootstrapStopsOnError(t *testing.T) {
	reg := NewRegistry()
	errBoom := errors.New("boom")
	app1 := &failingApp{name: "bad", failOn: "register", err: errBoom}
	app2 := &trackingApp{name: "never"}
	reg.Add(app1)
	reg.Add(app2)

	err := reg.Bootstrap(nil)
	require.ErrorIs(t, err, errBoom)
	assert.Contains(t, err.Error(), "bad")
	assert.False(t, app2.registered)
}

func TestRegistryAllNavItemsSortedByPosition(t *testing.T) {
	reg := NewRegistry()
	reg.Add(&trackingApp{
		name: "app1",
		navItems: []NavItem{
			{Label: "Settings", Position: 30},
			{Label: "Dashboard", Position: 10},
		},
	})
	reg.Add(&trackingApp{
		name: "app2",
		navItems: []NavItem{
			{Label: "Profile", Position: 20},
		},
	})
	// minimalApp doesn't implement HasNavItems - should be skipped.
	reg.Add(&minimalApp{})

	items := reg.AllNavItems()
	require.Len(t, items, 3)
	assert.Equal(t, "Dashboard", items[0].Label)
	assert.Equal(t, "Profile", items[1].Label)
	assert.Equal(t, "Settings", items[2].Label)
}

func TestRegistryAllNavItemsEmpty(t *testing.T) {
	reg := NewRegistry()
	reg.Add(&minimalApp{})

	items := reg.AllNavItems()
	assert.Empty(t, items)
}

func TestRegistryRegisterMiddleware(t *testing.T) {
	reg := NewRegistry()

	called := false
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			next.ServeHTTP(w, r)
		})
	}
	reg.Add(&trackingApp{name: "mw-app", middleware: []func(http.Handler) http.Handler{mw}})
	reg.Add(&minimalApp{}) // No middleware, should be skipped.

	r := chi.NewRouter()
	reg.RegisterMiddleware(r)

	r.Get("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.True(t, called)
}

func TestRegistryAllFlags(t *testing.T) {
	reg := NewRegistry()
	flag1 := &cli.StringFlag{Name: "auth-key"}
	flag2 := &cli.BoolFlag{Name: "debug"}
	reg.Add(&trackingApp{name: "app1", flags: []cli.Flag{flag1}})
	reg.Add(&trackingApp{name: "app2", flags: []cli.Flag{flag2}})
	reg.Add(&minimalApp{}) // Not Configurable, should be skipped.

	flags := reg.AllFlags()
	require.Len(t, flags, 2)
	assert.Equal(t, flag1, flags[0])
	assert.Equal(t, flag2, flags[1])
}

func TestRegistryConfigureCallsConfigurableApps(t *testing.T) {
	reg := NewRegistry()
	app1 := &trackingApp{name: "conf1"}
	app2 := &trackingApp{name: "conf2"}
	reg.Add(app1)
	reg.Add(app2)
	reg.Add(&minimalApp{}) // Not Configurable, should be skipped.

	err := reg.Configure(nil)
	require.NoError(t, err)
	assert.True(t, app1.configured)
	assert.True(t, app2.configured)
}

func TestRegistryConfigureStopsOnError(t *testing.T) {
	reg := NewRegistry()
	errCfg := errors.New("config error")
	reg.Add(&failingApp{name: "bad-cfg", failOn: "configure", err: errCfg})

	err := reg.Configure(nil)
	require.ErrorIs(t, err, errCfg)
	assert.Contains(t, err.Error(), "bad-cfg")
}

func TestRegistryAllCLICommands(t *testing.T) {
	reg := NewRegistry()
	cmd1 := &cli.Command{Name: "migrate"}
	cmd2 := &cli.Command{Name: "seed"}
	reg.Add(&trackingApp{name: "app1", commands: []*cli.Command{cmd1}})
	reg.Add(&trackingApp{name: "app2", commands: []*cli.Command{cmd2}})
	reg.Add(&minimalApp{}) // No commands, should be skipped.

	cmds := reg.AllCLICommands()
	require.Len(t, cmds, 2)
	assert.Equal(t, "migrate", cmds[0].Name)
	assert.Equal(t, "seed", cmds[1].Name)
}

func TestRegistrySeedCallsSeedableApps(t *testing.T) {
	reg := NewRegistry()
	app1 := &trackingApp{name: "s1"}
	app2 := &trackingApp{name: "s2"}
	reg.Add(app1)
	reg.Add(app2)
	reg.Add(&minimalApp{}) // Not Seedable, should be skipped.

	err := reg.Seed(context.Background())
	require.NoError(t, err)
	assert.True(t, app1.seeded)
	assert.True(t, app2.seeded)
}

func TestRegistrySeedStopsOnError(t *testing.T) {
	reg := NewRegistry()
	errSeed := errors.New("seed error")
	bad := &failingApp{name: "bad-seed", failOn: "seed", err: errSeed}
	unreached := &failingApp{name: "unreached", failOn: "seed", err: errors.New("should not happen")}
	reg.Add(bad)
	reg.Add(unreached)

	err := reg.Seed(context.Background())
	require.ErrorIs(t, err, errSeed)
	assert.Contains(t, err.Error(), "bad-seed")
	assert.False(t, unreached.reached)
}

func TestRegistryRegisterRoutes(t *testing.T) {
	reg := NewRegistry()

	routeApp := &routeApp{name: "router"}
	reg.Add(routeApp)
	reg.Add(&minimalApp{}) // No routes, should be skipped.

	r := chi.NewRouter()
	reg.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/from-app", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "route-registered", rec.Body.String())
}

// routeApp is a test helper implementing App + HasRoutes.
type routeApp struct {
	name string
}

func (a *routeApp) Name() string                { return a.name }
func (a *routeApp) Register(_ *AppConfig) error { return nil }
func (a *routeApp) Routes(r chi.Router) {
	r.Get("/from-app", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("route-registered"))
	})
}

func TestRegistryAllAdminNavItemsSortedByPosition(t *testing.T) {
	reg := NewRegistry()
	reg.Add(&trackingApp{
		name: "app1",
		adminNavItems: []NavItem{
			{Label: "Users", Position: 10},
			{Label: "Settings", Position: 30},
		},
	})
	reg.Add(&trackingApp{
		name: "app2",
		adminNavItems: []NavItem{
			{Label: "Invites", Position: 20},
		},
	})
	// minimalApp doesn't implement HasAdmin - but trackingApp always does.
	// Add a minimalApp to test it is skipped.
	reg.Add(&minimalApp{})

	items := reg.AllAdminNavItems()
	require.Len(t, items, 3)
	assert.Equal(t, "Users", items[0].Label)
	assert.Equal(t, "Invites", items[1].Label)
	assert.Equal(t, "Settings", items[2].Label)
}

func TestRegistryAllAdminNavItemsEmpty(t *testing.T) {
	reg := NewRegistry()
	reg.Add(&minimalApp{})

	items := reg.AllAdminNavItems()
	assert.Empty(t, items)
}

func TestNavItemsSortStable(t *testing.T) {
	items := []NavItem{
		{Label: "B", Position: 10},
		{Label: "A", Position: 10},
		{Label: "C", Position: 5},
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Position < items[j].Position
	})
	assert.Equal(t, "C", items[0].Label)
	// Same position: insertion order preserved by stable sort.
	assert.Equal(t, "B", items[1].Label)
	assert.Equal(t, "A", items[2].Label)
}
