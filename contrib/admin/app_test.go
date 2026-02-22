package admin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/admin/templates"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"codeberg.org/oliverandrich/burrow/contrib/session"
	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ burrow.App             = (*App)(nil)
	_ burrow.HasRoutes       = (*App)(nil)
	_ burrow.HasMiddleware   = (*App)(nil)
	_ burrow.HasDependencies = (*App)(nil)
)

func TestAppName(t *testing.T) {
	app := New(nil)
	assert.Equal(t, "admin", app.Name())
}

func TestAppDependencies(t *testing.T) {
	app := New(nil)
	assert.Equal(t, []string{"auth"}, app.Dependencies())
}

func TestAppRegister(t *testing.T) {
	app := New(nil)
	registry := burrow.NewRegistry()

	registry.Add(&session.App{})
	authApp := auth.New(nil)
	registry.Add(authApp)
	require.NoError(t, registry.Bootstrap(nil))

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
		func() { registry.Add(New(nil)) },
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

	registry.Add(&session.App{})
	authApp := auth.New(nil)
	registry.Add(authApp)
	require.NoError(t, registry.Bootstrap(nil))

	provider := &hasAdminApp{}
	registry.Add(provider)

	app := New(nil)
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

	registry.Add(&session.App{})
	authApp := auth.New(nil)
	registry.Add(authApp)
	require.NoError(t, registry.Bootstrap(nil))

	provider := &hasAdminApp{}
	registry.Add(provider)

	app := New(nil)
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

	registry.Add(&session.App{})
	authApp := auth.New(nil)
	registry.Add(authApp)
	require.NoError(t, registry.Bootstrap(nil))

	provider := &hasAdminApp{}
	registry.Add(provider)

	app := New(nil)
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
	app := New(nil)
	r := chi.NewRouter()
	// Routes should not panic when registry is nil.
	assert.NotPanics(t, func() { app.Routes(r) })
}

func TestMiddlewareInjectsAdminNavItems(t *testing.T) {
	registry := burrow.NewRegistry()

	registry.Add(&session.App{})
	authApp := auth.New(nil)
	registry.Add(authApp)
	require.NoError(t, registry.Bootstrap(nil))

	provider := &hasAdminApp{}
	registry.Add(provider)

	app := New(nil)
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	mws := app.Middleware()
	require.Len(t, mws, 1)

	var got []burrow.NavItem
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = NavItems(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := mws[0](inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotEmpty(t, got)
	// The test provider contributes "Test Resource"; auth contributes others.
	var found bool
	for _, item := range got {
		if item.Label == "Test Resource" {
			found = true
			break
		}
	}
	assert.True(t, found, "admin nav items should include items from HasAdmin apps")
}

func TestNewWithLayout(t *testing.T) {
	layout := burrow.LayoutFunc(func(_ string, content templ.Component) templ.Component {
		return content
	})

	app := New(layout)
	assert.NotNil(t, app.layout)
}

func TestNewWithoutLayout(t *testing.T) {
	app := New(nil)
	assert.Nil(t, app.layout)
}

func TestAdminIndexPage(t *testing.T) {
	registry := burrow.NewRegistry()

	registry.Add(&session.App{})
	authApp := auth.New(nil)
	registry.Add(authApp)
	require.NoError(t, registry.Bootstrap(nil))

	app := New(Layout())
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithUser(r.Context(), &auth.User{ID: 1, Role: auth.RoleAdmin})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	for _, mw := range app.Middleware() {
		r.Use(mw)
	}
	app.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	html := string(body)
	assert.Contains(t, html, "<!doctype html>")
	assert.Contains(t, html, "<title>Admin – Admin</title>")
	assert.Contains(t, html, `<li class="breadcrumb-item active" aria-current="page">Admin</li>`)
}

func TestAdminIndexPageRendersSidebar(t *testing.T) {
	registry := burrow.NewRegistry()

	registry.Add(&session.App{})
	authApp := auth.New(nil)
	registry.Add(authApp)
	require.NoError(t, registry.Bootstrap(nil))

	provider := &hasAdminApp{}
	registry.Add(provider)

	app := New(Layout())
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithUser(r.Context(), &auth.User{ID: 1, Role: auth.RoleAdmin})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	for _, mw := range app.Middleware() {
		r.Use(mw)
	}
	app.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	html := string(body)
	// Nav items should appear in the sidebar.
	assert.Contains(t, html, `href="/admin/test-resource"`)
	assert.Contains(t, html, "Test Resource")
	// The sidebar should contain the collapsible group.
	assert.Contains(t, html, `data-bs-toggle="collapse"`)
	assert.Contains(t, html, `id="collapse-test-admin-provider"`)
}

func TestLayout(t *testing.T) {
	layout := Layout()
	require.NotNil(t, layout)

	content := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, "<p>test content</p>")
		return err
	})

	component := layout("Test Page", content)
	require.NotNil(t, component)

	var buf strings.Builder
	err := component.Render(context.Background(), &buf)
	require.NoError(t, err)

	html := buf.String()
	assert.Contains(t, html, "<!doctype html>")
	assert.Contains(t, html, "<title>Test Page – Admin</title>")
	assert.Contains(t, html, "bootstrap.min.css")
	assert.Contains(t, html, "bootstrap-icons.min.css")
	assert.Contains(t, html, "bootstrap.bundle.min.js")
	assert.Contains(t, html, "htmx.min.js")
	assert.Contains(t, html, "<p>test content</p>")
}

func TestMiddlewareDoesNotInjectLayout(t *testing.T) {
	layout := burrow.LayoutFunc(func(_ string, content templ.Component) templ.Component {
		return content
	})

	registry := burrow.NewRegistry()
	registry.Add(&session.App{})
	authApp := auth.New(nil)
	registry.Add(authApp)
	require.NoError(t, registry.Bootstrap(nil))

	app := New(layout)
	registry.Add(app)
	require.NoError(t, app.Register(&burrow.AppConfig{Registry: registry}))

	mws := app.Middleware()

	var got burrow.LayoutFunc
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = burrow.Layout(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := mws[0](inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Nil(t, got, "middleware should not inject layout (layout is injected in route group)")
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
	layout := burrow.LayoutFunc(func(_ string, content templ.Component) templ.Component {
		return content
	})

	registry := burrow.NewRegistry()
	registry.Add(&session.App{})
	authApp := auth.New(nil)
	registry.Add(authApp)
	require.NoError(t, registry.Bootstrap(nil))

	checker := &layoutCheckApp{}
	registry.Add(checker)

	app := New(layout)
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

	registry.Add(&session.App{})
	authApp := auth.New(nil)
	registry.Add(authApp)
	require.NoError(t, registry.Bootstrap(nil))

	provider := &hasAdminApp{}
	registry.Add(provider)

	app := New(nil)
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
	gotGroups []templates.NavGroup
}

func (a *navGroupsCheckApp) Name() string                       { return "nav-groups-check" }
func (a *navGroupsCheckApp) Register(_ *burrow.AppConfig) error { return nil }
func (a *navGroupsCheckApp) AdminRoutes(r chi.Router) {
	r.Get("/nav-groups-check", func(w http.ResponseWriter, r *http.Request) {
		a.gotGroups = templates.NavGroupsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}
func (a *navGroupsCheckApp) AdminNavItems() []burrow.NavItem {
	return []burrow.NavItem{{Label: "Check", URL: "/admin/nav-groups-check"}}
}

func TestRoutesInjectNavGroups(t *testing.T) {
	registry := burrow.NewRegistry()
	registry.Add(&session.App{})
	authApp := auth.New(nil)
	registry.Add(authApp)
	require.NoError(t, registry.Bootstrap(nil))

	checker := &navGroupsCheckApp{}
	registry.Add(checker)

	app := New(nil)
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
