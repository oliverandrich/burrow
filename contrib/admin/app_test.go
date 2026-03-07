package admin

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"codeberg.org/oliverandrich/burrow/contrib/session"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ burrow.App             = (*App)(nil)
	_ burrow.HasRoutes       = (*App)(nil)
	_ burrow.HasDependencies = (*App)(nil)
	_ burrow.HasTemplates    = (*App)(nil)
	_ burrow.HasFuncMap      = (*App)(nil)
)

func TestAppName(t *testing.T) {
	app := New()
	assert.Equal(t, "admin", app.Name())
}

func TestAppDependencies(t *testing.T) {
	app := New()
	assert.Equal(t, []string{"auth"}, app.Dependencies())
}

func TestAppRegister(t *testing.T) {
	app := New()
	registry := burrow.NewRegistry()

	registry.Add(session.New())
	authApp := auth.New()
	registry.Add(authApp)
	require.NoError(t, registry.RegisterAll(nil))

	err := app.Register(&burrow.AppConfig{
		Registry: registry,
	})

	require.NoError(t, err)
	assert.NotNil(t, app.registry)
}

func TestAppRegisterMissingAuthPanics(t *testing.T) {
	registry := burrow.NewRegistry()

	assert.PanicsWithValue(t,
		`burrow: app "admin" requires "auth" to be registered first`,
		func() { registry.Add(New()) },
	)
}

// hasAdminApp is a test app implementing HasAdmin.
type hasAdminApp struct {
	routesCalled bool
}

func (a *hasAdminApp) Name() string                       { return "test-admin-provider" }
func (a *hasAdminApp) Register(_ *burrow.AppConfig) error { return nil }
func (a *hasAdminApp) AdminRoutes(r chi.Router) {
	a.routesCalled = true
	r.Get("/test-resource", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test-admin-page"))
	})
}
func (a *hasAdminApp) AdminNavItems() []burrow.NavItem {
	return []burrow.NavItem{
		{Label: "Test Resource", URL: "/admin/test-resource", Position: 50},
	}
}

func TestRoutesCoordinatesHasAdminApps(t *testing.T) {
	registry := burrow.NewRegistry()

	registry.Add(session.New())
	authApp := auth.New()
	registry.Add(authApp)
	require.NoError(t, registry.RegisterAll(nil))

	provider := &hasAdminApp{}
	registry.Add(provider)

	app := New()
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	r := chi.NewRouter()
	// Inject admin user for RequireAuth + RequireAdmin.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithUser(r.Context(), &auth.User{ID: 1, Role: auth.RoleAdmin})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	app.Routes(r)

	assert.True(t, provider.routesCalled, "AdminRoutes should be called on HasAdmin apps")

	req := httptest.NewRequest(http.MethodGet, "/admin/test-resource", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test-admin-page", rec.Body.String())
}

func TestRoutesRequiresAuth(t *testing.T) {
	registry := burrow.NewRegistry()

	registry.Add(session.New())
	authApp := auth.New()
	registry.Add(authApp)
	require.NoError(t, registry.RegisterAll(nil))

	provider := &hasAdminApp{}
	registry.Add(provider)

	app := New()
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	r := chi.NewRouter()
	app.Routes(r)

	// Unauthenticated request should redirect to login.
	req := httptest.NewRequest(http.MethodGet, "/admin/test-resource", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "/auth/login")
}

func TestRoutesRequiresAdmin(t *testing.T) {
	registry := burrow.NewRegistry()

	registry.Add(session.New())
	authApp := auth.New()
	registry.Add(authApp)
	require.NoError(t, registry.RegisterAll(nil))

	provider := &hasAdminApp{}
	registry.Add(provider)

	app := New()
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	r := chi.NewRouter()
	// Inject non-admin user.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithUser(r.Context(), &auth.User{ID: 1, Role: auth.RoleUser})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	app.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/admin/test-resource", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRoutesNoRegistryNoPanic(t *testing.T) {
	app := New()
	r := chi.NewRouter()
	// Routes should not panic when registry is nil.
	assert.NotPanics(t, func() { app.Routes(r) })
}

func TestNewWithLayout(t *testing.T) {
	layout := burrow.LayoutFunc(func(_ http.ResponseWriter, _ *http.Request, _ int, content template.HTML, _ map[string]any) error {
		return nil
	})

	app := New(WithLayout(layout))
	assert.NotNil(t, app.layout)
}

func TestNewWithoutLayout(t *testing.T) {
	app := New()
	assert.Nil(t, app.layout)
}

// mockDashboardRenderer is a mock DashboardRenderer for testing.
type mockDashboardRenderer struct {
	called bool
}

func (m *mockDashboardRenderer) DashboardPage(w http.ResponseWriter, _ *http.Request) error {
	m.called = true
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("mock-dashboard"))
	return nil
}

func TestIndexPageWithDashboardRenderer(t *testing.T) {
	mock := &mockDashboardRenderer{}
	app := New(WithDashboardRenderer(mock))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	err := app.indexPage(rec, req)

	require.NoError(t, err)
	assert.True(t, mock.called)
	assert.Equal(t, "mock-dashboard", rec.Body.String())
}

func TestIndexPageWithoutDashboardRenderer(t *testing.T) {
	app := New()

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	err := app.indexPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, "admin dashboard", rec.Body.String())
}

// layoutCheckApp captures burrow.Layout from context inside the /admin group.
type layoutCheckApp struct {
	gotLayout burrow.LayoutFunc
}

func (a *layoutCheckApp) Name() string                       { return "layout-check" }
func (a *layoutCheckApp) Register(_ *burrow.AppConfig) error { return nil }
func (a *layoutCheckApp) AdminRoutes(r chi.Router) {
	r.Get("/layout-check", func(w http.ResponseWriter, r *http.Request) {
		a.gotLayout = burrow.Layout(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}
func (a *layoutCheckApp) AdminNavItems() []burrow.NavItem { return nil }

func TestRoutesInjectLayoutInGroup(t *testing.T) {
	layout := burrow.LayoutFunc(func(_ http.ResponseWriter, _ *http.Request, _ int, content template.HTML, _ map[string]any) error {
		return nil
	})

	registry := burrow.NewRegistry()
	registry.Add(session.New())
	authApp := auth.New()
	registry.Add(authApp)
	require.NoError(t, registry.RegisterAll(nil))

	checker := &layoutCheckApp{}
	registry.Add(checker)

	app := New(WithLayout(layout))
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	r := chi.NewRouter()
	// Inject admin user for RequireAuth + RequireAdmin.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithUser(r.Context(), &auth.User{ID: 1, Role: auth.RoleAdmin})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	app.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/admin/layout-check", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotNil(t, checker.gotLayout, "admin layout should be set in context inside /admin route group")
}

func TestBuildNavGroups(t *testing.T) {
	registry := burrow.NewRegistry()

	registry.Add(session.New())
	authApp := auth.New()
	registry.Add(authApp)
	require.NoError(t, registry.RegisterAll(nil))

	provider := &hasAdminApp{}
	registry.Add(provider)

	app := New()
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	groups := app.buildNavGroups()

	// auth contributes admin nav items, and so does hasAdminApp.
	require.GreaterOrEqual(t, len(groups), 2)

	names := make([]string, 0, len(groups))
	for _, g := range groups {
		names = append(names, g.AppName)
	}
	assert.Contains(t, names, "auth")
	assert.Contains(t, names, "test-admin-provider")
}

// navGroupsCheckApp captures nav groups from context inside the /admin group.
type navGroupsCheckApp struct {
	gotGroups []NavGroup
}

func (a *navGroupsCheckApp) Name() string                       { return "nav-groups-check" }
func (a *navGroupsCheckApp) Register(_ *burrow.AppConfig) error { return nil }
func (a *navGroupsCheckApp) AdminRoutes(r chi.Router) {
	r.Get("/nav-groups-check", func(w http.ResponseWriter, r *http.Request) {
		a.gotGroups = NavGroupsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}
func (a *navGroupsCheckApp) AdminNavItems() []burrow.NavItem {
	return []burrow.NavItem{{Label: "Check", URL: "/admin/nav-groups-check"}}
}

func TestRoutesInjectNavGroups(t *testing.T) {
	registry := burrow.NewRegistry()
	registry.Add(session.New())
	authApp := auth.New()
	registry.Add(authApp)
	require.NoError(t, registry.RegisterAll(nil))

	checker := &navGroupsCheckApp{}
	registry.Add(checker)

	app := New()
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithUser(r.Context(), &auth.User{ID: 1, Role: auth.RoleAdmin})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	app.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/admin/nav-groups-check", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotEmpty(t, checker.gotGroups, "nav groups should be injected in /admin route group")

	names := make([]string, 0, len(checker.gotGroups))
	for _, g := range checker.gotGroups {
		names = append(names, g.AppName)
	}
	assert.Contains(t, names, "nav-groups-check")
}
