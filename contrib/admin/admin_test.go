package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/oliverandrich/go-webapp-template/contrib/auth"
	"codeberg.org/oliverandrich/go-webapp-template/core"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ core.App            = (*App)(nil)
	_ core.HasNavItems    = (*App)(nil)
	_ core.HasRoutes      = (*App)(nil)
	_ core.HasCLICommands = (*App)(nil)
)

func TestAppName(t *testing.T) {
	app := New()
	assert.Equal(t, "admin", app.Name())
}

func TestAppRegister(t *testing.T) {
	app := New()
	registry := core.NewRegistry()

	// Register auth app and bootstrap it so Repo() is available.
	authApp := auth.New(nil)
	registry.Add(authApp)
	require.NoError(t, registry.Bootstrap(nil))

	err := app.Register(&core.AppConfig{
		Registry: registry,
	})

	require.NoError(t, err)
	// authRepo is set only if auth app was bootstrapped with a DB.
	// Without a DB, Repo() returns a repo with nil DB, which is still not nil.
	assert.NotNil(t, app.authRepo)
}

func TestAppRegisterMissingAuth(t *testing.T) {
	app := New()
	registry := core.NewRegistry()

	err := app.Register(&core.AppConfig{
		Registry: registry,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth")
}

func TestNavItems(t *testing.T) {
	app := New()
	items := app.NavItems()

	require.NotEmpty(t, items)
	found := false
	for _, item := range items {
		if item.URL == "/admin/users" {
			found = true
			assert.True(t, item.AdminOnly)
		}
	}
	assert.True(t, found, "should have /admin/users nav item")
}

func TestCLICommands(t *testing.T) {
	app := New()
	cmds := app.CLICommands()

	require.NotEmpty(t, cmds)

	names := make(map[string]bool)
	for _, cmd := range cmds {
		names[cmd.Name] = true
	}

	assert.True(t, names["promote"], "should have promote command")
	assert.True(t, names["demote"], "should have demote command")
	assert.True(t, names["create-invite"], "should have create-invite command")
}

func TestRoutesNoHandlers(t *testing.T) {
	app := New()
	e := echo.New()
	// Routes should not panic when handlers are nil.
	assert.NotPanics(t, func() { app.Routes(e) })
}

func TestRoutesWithHandlers(t *testing.T) {
	app := New()
	store := &mockStore{users: []auth.User{}}
	renderer := &mockRenderer{}
	app.handlers = NewHandlers(store, renderer)

	e := echo.New()
	// Inject admin user into context for RequireAuth + RequireAdmin.
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			auth.SetUser(c, &auth.User{ID: 1, Role: auth.RoleAdmin})
			return next(c)
		}
	})
	app.Routes(e)

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- Handler tests ---

type mockStore struct { //nolint:govet // fieldalignment: readability over optimization
	users      []auth.User
	user       *auth.User
	err        error
	roleCalled bool
	lastRole   string
}

func (m *mockStore) ListUsers(_ context.Context) ([]auth.User, error) {
	return m.users, m.err
}

func (m *mockStore) GetUserByID(_ context.Context, _ int64) (*auth.User, error) {
	if m.user == nil && m.err == nil {
		return nil, auth.ErrNotFound
	}
	return m.user, m.err
}

func (m *mockStore) SetUserRole(_ context.Context, _ int64, role string) error {
	m.roleCalled = true
	m.lastRole = role
	return m.err
}

func (m *mockStore) GetUserByUsername(_ context.Context, _ string) (*auth.User, error) {
	if m.user == nil && m.err == nil {
		return nil, auth.ErrNotFound
	}
	return m.user, m.err
}

func (m *mockStore) CreateInvite(_ context.Context, _ *auth.Invite) error {
	return m.err
}

type mockRenderer struct {
	lastMethod string
}

func (m *mockRenderer) UsersPage(c *echo.Context, _ []auth.User) error {
	m.lastMethod = "UsersPage"
	return c.String(http.StatusOK, "users")
}

func (m *mockRenderer) UserDetailPage(c *echo.Context, _ *auth.User) error {
	m.lastMethod = "UserDetailPage"
	return c.String(http.StatusOK, "user-detail")
}

func newTestHandlers() (*Handlers, *mockStore, *mockRenderer) {
	store := &mockStore{}
	renderer := &mockRenderer{}
	h := NewHandlers(store, renderer)
	return h, store, renderer
}

func TestUsersPageHandler(t *testing.T) {
	h, store, r := newTestHandlers()
	store.users = []auth.User{
		{ID: 1, Username: "alice", Role: auth.RoleAdmin},
		{ID: 2, Username: "bob", Role: auth.RoleUser},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.UsersPage(c)

	require.NoError(t, err)
	assert.Equal(t, "UsersPage", r.lastMethod)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUsersPageHandlerError(t *testing.T) {
	h, store, _ := newTestHandlers()
	store.err = assert.AnError

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.UsersPage(c)

	require.Error(t, err)
}

func TestUserDetailHandler(t *testing.T) {
	h, store, r := newTestHandlers()
	store.user = &auth.User{ID: 1, Username: "alice", Role: auth.RoleAdmin}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/users/1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "1"}})

	err := h.UserDetail(c)

	require.NoError(t, err)
	assert.Equal(t, "UserDetailPage", r.lastMethod)
}

func TestUserDetailNotFound(t *testing.T) {
	h, _, _ := newTestHandlers()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/users/999", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "999"}})

	err := h.UserDetail(c)

	require.Error(t, err)
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusNotFound, httpErr.Code)
}

func TestUserDetailInvalidID(t *testing.T) {
	h, _, _ := newTestHandlers()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/users/abc", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "abc"}})

	err := h.UserDetail(c)

	require.Error(t, err)
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
}

func TestUpdateUserRolePromote(t *testing.T) {
	h, store, _ := newTestHandlers()
	store.user = &auth.User{ID: 1, Username: "alice", Role: auth.RoleUser}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/admin/users/1/role", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	req.Form = map[string][]string{"role": {auth.RoleAdmin}}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "1"}})

	err := h.UpdateUserRole(c)

	require.NoError(t, err)
	assert.True(t, store.roleCalled)
	assert.Equal(t, auth.RoleAdmin, store.lastRole)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
}

func TestUpdateUserRoleDemote(t *testing.T) {
	h, store, _ := newTestHandlers()
	store.user = &auth.User{ID: 1, Username: "alice", Role: auth.RoleAdmin}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/admin/users/1/role", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	req.Form = map[string][]string{"role": {auth.RoleUser}}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "1"}})

	err := h.UpdateUserRole(c)

	require.NoError(t, err)
	assert.True(t, store.roleCalled)
	assert.Equal(t, auth.RoleUser, store.lastRole)
}

func TestUpdateUserRoleInvalidRole(t *testing.T) {
	h, store, _ := newTestHandlers()
	store.user = &auth.User{ID: 1, Username: "alice", Role: auth.RoleUser}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/admin/users/1/role", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	req.Form = map[string][]string{"role": {"superadmin"}}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "1"}})

	err := h.UpdateUserRole(c)

	require.Error(t, err)
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
	assert.False(t, store.roleCalled)
}

func TestUpdateUserRoleNotFound(t *testing.T) {
	h, _, _ := newTestHandlers()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/admin/users/999/role", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	req.Form = map[string][]string{"role": {auth.RoleAdmin}}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "999"}})

	err := h.UpdateUserRole(c)

	require.Error(t, err)
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusNotFound, httpErr.Code)
}
