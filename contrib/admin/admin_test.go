package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/oliverandrich/burrow"
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

func TestNavItemsContext(t *testing.T) {
	ctx := context.Background()
	items := []burrow.NavItem{
		{Label: "Users", URL: "/admin/users", Position: 10},
		{Label: "Invites", URL: "/admin/invites", Position: 20},
	}

	ctx = WithNavItems(ctx, items)
	got := NavItems(ctx)

	require.Len(t, got, 2)
	assert.Equal(t, "Users", got[0].Label)
	assert.Equal(t, "Invites", got[1].Label)
}

func TestNavItemsMissing(t *testing.T) {
	ctx := context.Background()
	assert.Nil(t, NavItems(ctx))
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

func TestLayoutContext(t *testing.T) {
	layout := burrow.LayoutFunc(func(_ string, content templ.Component) templ.Component {
		return content
	})

	ctx := context.Background()
	ctx = WithLayout(ctx, layout)

	got := Layout(ctx)
	assert.NotNil(t, got)
}

func TestLayoutMissing(t *testing.T) {
	ctx := context.Background()
	assert.Nil(t, Layout(ctx))
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

func TestMiddlewareInjectsLayout(t *testing.T) {
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
		got = Layout(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := mws[0](inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.NotNil(t, got, "admin layout should be set in context")
}
