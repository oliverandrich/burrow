package burrow

import (
	"bytes"
	"context"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"sort"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow/registry"
	"github.com/oliverandrich/den/document"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

// minimalApp implements only the required App interface.
type minimalApp struct{}

func (a *minimalApp) Name() string { return "minimal" }

// fullApp implements App + all optional interfaces.
type fullApp struct {
	configured bool
}

func (a *fullApp) Name() string                                      { return "full" }
func (a *fullApp) Documents() []document.Document                    { return nil }
func (a *fullApp) Middleware() []func(http.Handler) http.Handler     { return nil }
func (a *fullApp) NavItems() []NavItem                               { return nil }
func (a *fullApp) Flags(_ func(string) cli.ValueSource) []cli.Flag   { return nil }
func (a *fullApp) Configure(_ *AppConfig, _ *cli.Command) error      { a.configured = true; return nil }
func (a *fullApp) CLICommands() []*cli.Command                       { return nil }
func (a *fullApp) Migrations() []NamedMigration                      { return nil }
func (a *fullApp) Routes(_ chi.Router)                               {}
func (a *fullApp) AdminRoutes(_ chi.Router)                          {}
func (a *fullApp) AdminNavItems() []NavItem                          { return nil }
func (a *fullApp) TemplateFS() fs.FS                                 { return nil }
func (a *fullApp) FuncMap() template.FuncMap                         { return nil }
func (a *fullApp) RequestFuncMap(_ context.Context) template.FuncMap { return nil }
func (a *fullApp) Start(_ *Server) error                             { return nil }

// trackingApp records calls and provides test data for lifecycle methods.
type trackingApp struct {
	configureFn   func(cfg *AppConfig) error
	name          string
	navItems      []NavItem
	adminNavItems []NavItem
	middleware    []func(http.Handler) http.Handler
	flags         []cli.Flag
	commands      []*cli.Command
	migrations    []NamedMigration
	configured    bool
}

func (a *trackingApp) Name() string                                    { return a.name }
func (a *trackingApp) NavItems() []NavItem                             { return a.navItems }
func (a *trackingApp) Middleware() []func(http.Handler) http.Handler   { return a.middleware }
func (a *trackingApp) Flags(_ func(string) cli.ValueSource) []cli.Flag { return a.flags }
func (a *trackingApp) Configure(cfg *AppConfig, _ *cli.Command) error {
	a.configured = true
	if a.configureFn != nil {
		return a.configureFn(cfg)
	}
	return nil
}
func (a *trackingApp) CLICommands() []*cli.Command  { return a.commands }
func (a *trackingApp) Migrations() []NamedMigration { return a.migrations }
func (a *trackingApp) AdminRoutes(_ chi.Router)     {}
func (a *trackingApp) AdminNavItems() []NavItem     { return a.adminNavItems }

// failingApp returns the configured error from Configure (or nil when err is nil).
type failingApp struct {
	err  error
	name string
}

func (a *failingApp) Name() string                                    { return a.name }
func (a *failingApp) Flags(_ func(string) cli.ValueSource) []cli.Flag { return nil }
func (a *failingApp) Configure(_ *AppConfig, _ *cli.Command) error    { return a.err }

// dependentApp implements App + HasDependencies with a configurable name.
type dependentApp struct {
	name string
	deps []string
}

func (a *dependentApp) Name() string           { return a.name }
func (a *dependentApp) Dependencies() []string { return a.deps }

// Compile-time interface assertions.
var (
	_ App               = (*minimalApp)(nil)
	_ App               = (*fullApp)(nil)
	_ HasDocuments      = (*fullApp)(nil)
	_ HasMiddleware     = (*fullApp)(nil)
	_ HasNavItems       = (*fullApp)(nil)
	_ HasFlags          = (*fullApp)(nil)
	_ Configurable      = (*fullApp)(nil)
	_ HasCLICommands    = (*fullApp)(nil)
	_ HasMigrations     = (*fullApp)(nil)
	_ HasRoutes         = (*fullApp)(nil)
	_ HasAdmin          = (*fullApp)(nil)
	_ HasDependencies   = (*dependentApp)(nil)
	_ HasTemplates      = (*fullApp)(nil)
	_ HasFuncMap        = (*fullApp)(nil)
	_ HasRequestFuncMap = (*fullApp)(nil)
	_ Startable         = (*fullApp)(nil)
)

func TestMinimalAppSatisfiesOnlyApp(t *testing.T) {
	var app App = &minimalApp{}
	assert.Equal(t, "minimal", app.Name())

	_, hasDocuments := app.(HasDocuments)
	_, hasMiddleware := app.(HasMiddleware)
	_, hasNavItems := app.(HasNavItems)
	_, isConfigurable := app.(Configurable)
	_, hasCLI := app.(HasCLICommands)
	_, hasMigrations := app.(HasMigrations)
	_, hasRoutes := app.(HasRoutes)
	_, hasAdmin := app.(HasAdmin)

	assert.False(t, hasDocuments)
	assert.False(t, hasMiddleware)
	assert.False(t, hasNavItems)
	assert.False(t, isConfigurable)
	assert.False(t, hasCLI)
	assert.False(t, hasMigrations)
	assert.False(t, hasRoutes)
	assert.False(t, hasAdmin)
}

func TestFullAppSatisfiesAllInterfaces(t *testing.T) {
	var app App = &fullApp{}

	_, hasDocuments := app.(HasDocuments)
	_, hasMiddleware := app.(HasMiddleware)
	_, hasNavItems := app.(HasNavItems)
	_, hasFlags := app.(HasFlags)
	_, isConfigurable := app.(Configurable)
	_, hasCLI := app.(HasCLICommands)
	_, hasMigrations := app.(HasMigrations)
	_, hasRoutes := app.(HasRoutes)
	_, hasAdmin := app.(HasAdmin)
	_, hasTemplates := app.(HasTemplates)
	_, hasFuncMap := app.(HasFuncMap)
	_, hasRequestFuncMap := app.(HasRequestFuncMap)
	_, isStartable := app.(Startable)

	assert.True(t, hasDocuments)
	assert.True(t, hasMiddleware)
	assert.True(t, hasNavItems)
	assert.True(t, hasFlags)
	assert.True(t, isConfigurable)
	assert.True(t, hasCLI)
	assert.True(t, hasMigrations)
	assert.True(t, hasRoutes)
	assert.True(t, hasAdmin)
	assert.True(t, hasTemplates)
	assert.True(t, hasFuncMap)
	assert.True(t, hasRequestFuncMap)
	assert.True(t, isStartable)
}

func TestNavItemFields(t *testing.T) {
	item := NavItem{
		Label:    "Dashboard",
		URL:      "/dashboard",
		Icon:     "app/icon_dashboard",
		Position: 10,
		AuthOnly: true,
	}

	assert.Equal(t, "Dashboard", item.Label)
	assert.Equal(t, "/dashboard", item.URL)
	assert.Equal(t, "app/icon_dashboard", item.Icon)
	assert.Equal(t, 10, item.Position)
	assert.True(t, item.AuthOnly)
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

	names := appNames(registry.Apps(srv.Registry()))
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

func indexOf(s []string, val string) int {
	for i, v := range s {
		if v == val {
			return i
		}
	}
	return -1
}
