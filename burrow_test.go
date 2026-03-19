package burrow

import (
	"bytes"
	"context"
	"errors"
	"html/template"
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

func (a *fullApp) Name() string                                    { return "full" }
func (a *fullApp) Register(_ *AppConfig) error                     { a.registered = true; return nil }
func (a *fullApp) MigrationFS() fs.FS                              { return nil }
func (a *fullApp) Middleware() []func(http.Handler) http.Handler   { return nil }
func (a *fullApp) NavItems() []NavItem                             { return nil }
func (a *fullApp) Flags(_ func(string) cli.ValueSource) []cli.Flag { return nil }
func (a *fullApp) Configure(_ *cli.Command) error                  { return nil }
func (a *fullApp) CLICommands() []*cli.Command                     { return nil }
func (a *fullApp) Seed(_ context.Context) error                    { return nil }
func (a *fullApp) Routes(_ chi.Router)                             {}
func (a *fullApp) AdminRoutes(_ chi.Router)                        {}
func (a *fullApp) AdminNavItems() []NavItem                        { return nil }
func (a *fullApp) TemplateFS() fs.FS                               { return nil }
func (a *fullApp) FuncMap() template.FuncMap                       { return nil }
func (a *fullApp) RequestFuncMap(_ *http.Request) template.FuncMap { return nil }

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
func (a *trackingApp) NavItems() []NavItem                             { return a.navItems }
func (a *trackingApp) Middleware() []func(http.Handler) http.Handler   { return a.middleware }
func (a *trackingApp) Flags(_ func(string) cli.ValueSource) []cli.Flag { return a.flags }
func (a *trackingApp) Configure(_ *cli.Command) error                  { a.configured = true; return nil }
func (a *trackingApp) CLICommands() []*cli.Command                     { return a.commands }
func (a *trackingApp) Seed(_ context.Context) error                    { a.seeded = true; return nil }
func (a *trackingApp) AdminRoutes(_ chi.Router)                        {}
func (a *trackingApp) AdminNavItems() []NavItem                        { return a.adminNavItems }

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
func (a *failingApp) Flags(_ func(string) cli.ValueSource) []cli.Flag { return nil }
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
	_ App               = (*minimalApp)(nil)
	_ App               = (*fullApp)(nil)
	_ Migratable        = (*fullApp)(nil)
	_ HasMiddleware     = (*fullApp)(nil)
	_ HasNavItems       = (*fullApp)(nil)
	_ Configurable      = (*fullApp)(nil)
	_ HasCLICommands    = (*fullApp)(nil)
	_ Seedable          = (*fullApp)(nil)
	_ HasRoutes         = (*fullApp)(nil)
	_ HasAdmin          = (*fullApp)(nil)
	_ HasDependencies   = (*dependentApp)(nil)
	_ HasTemplates      = (*fullApp)(nil)
	_ HasFuncMap        = (*fullApp)(nil)
	_ HasRequestFuncMap = (*fullApp)(nil)
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
	_, hasTemplates := app.(HasTemplates)
	_, hasFuncMap := app.(HasFuncMap)
	_, hasRequestFuncMap := app.(HasRequestFuncMap)

	assert.True(t, isMigratable)
	assert.True(t, hasMiddleware)
	assert.True(t, hasNavItems)
	assert.True(t, isConfigurable)
	assert.True(t, hasCLI)
	assert.True(t, isSeedable)
	assert.True(t, hasRoutes)
	assert.True(t, hasAdmin)
	assert.True(t, hasTemplates)
	assert.True(t, hasFuncMap)
	assert.True(t, hasRequestFuncMap)
}

func TestNavItemFields(t *testing.T) {
	item := NavItem{
		Label:    "Dashboard",
		URL:      "/dashboard",
		Icon:     template.HTML(`<svg>icon</svg>`),
		Position: 10,
		AuthOnly: true,
	}

	assert.Equal(t, "Dashboard", item.Label)
	assert.Equal(t, "/dashboard", item.URL)
	assert.Equal(t, template.HTML(`<svg>icon</svg>`), item.Icon)
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

// dependentApp implements App + HasDependencies with a configurable name.
type dependentApp struct {
	name string
	deps []string
}

func (a *dependentApp) Name() string                { return a.name }
func (a *dependentApp) Register(_ *AppConfig) error { return nil }
func (a *dependentApp) Dependencies() []string      { return a.deps }

func TestRegistryAddPanicsOnMissingDependency(t *testing.T) {
	reg := NewRegistry()

	assert.PanicsWithValue(t,
		`burrow: app "dependent" requires "session" to be registered first`,
		func() {
			reg.Add(&dependentApp{name: "dependent", deps: []string{"session"}})
		},
	)
}

func TestRegistryAddSucceedsWhenDependencySatisfied(t *testing.T) {
	reg := NewRegistry()
	reg.Add(&minimalApp{}) // name = "minimal"

	assert.NotPanics(t, func() {
		reg.Add(&dependentApp{name: "dependent", deps: []string{"minimal"}})
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

func TestRegistryRegisterAllCallsRegister(t *testing.T) {
	reg := NewRegistry()
	app1 := &trackingApp{name: "first"}
	app2 := &trackingApp{name: "second"}
	reg.Add(app1)
	reg.Add(app2)

	err := reg.RegisterAll(nil)
	require.NoError(t, err)
	assert.True(t, app1.registered)
	assert.True(t, app2.registered)
}

func TestRegistryRegisterAllPassesConfig(t *testing.T) {
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

	err := reg.RegisterAll(nil)
	require.NoError(t, err)
	require.NotNil(t, received)
	assert.Equal(t, reg, received.Registry)
}

func TestRegistryRegisterAllStopsOnError(t *testing.T) {
	reg := NewRegistry()
	errBoom := errors.New("boom")
	app1 := &failingApp{name: "bad", failOn: "register", err: errBoom}
	app2 := &trackingApp{name: "never"}
	reg.Add(app1)
	reg.Add(app2)

	err := reg.RegisterAll(nil)
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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
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

	flags := reg.AllFlags(nil)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/from-app", nil)
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

// --- sortApps tests ---

func appNames(apps []App) []string {
	names := make([]string, len(apps))
	for i, app := range apps {
		names[i] = app.Name()
	}
	return names
}

func TestSortApps_NoDependencies_PreservesOrder(t *testing.T) {
	apps := []App{
		&dependentApp{name: "a"},
		&dependentApp{name: "b"},
		&dependentApp{name: "c"},
	}

	sorted := sortApps(apps)

	assert.Equal(t, []string{"a", "b", "c"}, appNames(sorted))
}

func TestSortApps_ReordersDependencies(t *testing.T) {
	apps := []App{
		&dependentApp{name: "admin", deps: []string{"auth"}},
		&dependentApp{name: "auth", deps: []string{"session"}},
		&dependentApp{name: "session"},
	}

	sorted := sortApps(apps)

	names := appNames(sorted)
	// session must come before auth, auth must come before admin.
	assert.Equal(t, []string{"session", "auth", "admin"}, names)
}

func TestSortApps_PreservesRelativeOrderForIndependentApps(t *testing.T) {
	apps := []App{
		&dependentApp{name: "i18n"},
		&dependentApp{name: "staticfiles"},
		&dependentApp{name: "bootstrap", deps: []string{"staticfiles"}},
		&dependentApp{name: "healthcheck"},
	}

	sorted := sortApps(apps)

	names := appNames(sorted)
	// bootstrap must come after staticfiles; i18n and healthcheck keep relative order.
	assert.Less(t, indexOf(names, "staticfiles"), indexOf(names, "bootstrap"))
	assert.Less(t, indexOf(names, "i18n"), indexOf(names, "healthcheck"))
}

func TestSortApps_PanicsOnMissingDependency(t *testing.T) {
	apps := []App{
		&dependentApp{name: "auth", deps: []string{"session"}},
	}

	assert.PanicsWithValue(t,
		`burrow: app "auth" requires "session", but it was not provided`,
		func() { sortApps(apps) },
	)
}

func TestSortApps_PanicsOnCycle(t *testing.T) {
	apps := []App{
		&dependentApp{name: "a", deps: []string{"b"}},
		&dependentApp{name: "b", deps: []string{"a"}},
	}

	assert.Panics(t, func() { sortApps(apps) })
}

func TestSortApps_TransitiveDependencies(t *testing.T) {
	apps := []App{
		&dependentApp{name: "d", deps: []string{"c"}},
		&dependentApp{name: "c", deps: []string{"b"}},
		&dependentApp{name: "b", deps: []string{"a"}},
		&dependentApp{name: "a"},
	}

	sorted := sortApps(apps)

	assert.Equal(t, []string{"a", "b", "c", "d"}, appNames(sorted))
}

func TestSortApps_MultipleDependencies(t *testing.T) {
	apps := []App{
		&dependentApp{name: "admin", deps: []string{"auth", "bootstrap"}},
		&dependentApp{name: "auth", deps: []string{"session"}},
		&dependentApp{name: "bootstrap", deps: []string{"staticfiles"}},
		&dependentApp{name: "session"},
		&dependentApp{name: "staticfiles"},
	}

	sorted := sortApps(apps)

	names := appNames(sorted)
	assert.Less(t, indexOf(names, "session"), indexOf(names, "auth"))
	assert.Less(t, indexOf(names, "staticfiles"), indexOf(names, "bootstrap"))
	assert.Less(t, indexOf(names, "auth"), indexOf(names, "admin"))
	assert.Less(t, indexOf(names, "bootstrap"), indexOf(names, "admin"))
}

func TestNewServer_SortsAppsAutomatically(t *testing.T) {
	// Pass apps in wrong order — NewServer should sort them.
	srv := NewServer(
		&dependentApp{name: "admin", deps: []string{"auth"}},
		&dependentApp{name: "auth", deps: []string{"session"}},
		&dependentApp{name: "session"},
	)

	names := appNames(srv.Registry().Apps())
	assert.Equal(t, []string{"session", "auth", "admin"}, names)
}

func TestSortApps_LogsWarningWhenReordered(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(oldLogger) })

	apps := []App{
		&dependentApp{name: "auth", deps: []string{"session"}},
		&dependentApp{name: "session"},
	}

	sortApps(apps)

	output := buf.String()
	assert.Contains(t, output, "app registration order was adjusted")
	assert.Contains(t, output, "original")
	assert.Contains(t, output, "resolved")
}

func TestSortApps_NoWarningWhenOrderCorrect(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(oldLogger) })

	apps := []App{
		&dependentApp{name: "session"},
		&dependentApp{name: "auth", deps: []string{"session"}},
	}

	sortApps(apps)

	assert.NotContains(t, buf.String(), "app registration order was adjusted")
}

func TestAppConfig_RegisterIconFunc(t *testing.T) {
	cfg := &AppConfig{}
	icon := func(class ...string) template.HTML { return "<svg>test</svg>" }

	cfg.RegisterIconFunc("iconTest", icon)

	assert.Len(t, cfg.IconFuncs(), 1)
	assert.Contains(t, cfg.IconFuncs(), "iconTest")
}

func TestAppConfig_RegisterIconFunc_DuplicateIsNoop(t *testing.T) {
	cfg := &AppConfig{}
	icon1 := func(class ...string) template.HTML { return "<svg>first</svg>" }
	icon2 := func(class ...string) template.HTML { return "<svg>second</svg>" }

	cfg.RegisterIconFunc("iconTest", icon1)
	cfg.RegisterIconFunc("iconTest", icon2)

	assert.Len(t, cfg.IconFuncs(), 1)
	// First registration wins.
	fn := cfg.IconFuncs()["iconTest"]
	assert.Equal(t, template.HTML("<svg>first</svg>"), fn())
}

func TestAppConfig_IconFuncs_NilWhenEmpty(t *testing.T) {
	cfg := &AppConfig{}
	assert.Nil(t, cfg.IconFuncs())
}

func indexOf(s []string, val string) int {
	for i, v := range s {
		if v == val {
			return i
		}
	}
	return -1
}
