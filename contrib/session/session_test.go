package session

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

var testHashKey = hex.EncodeToString(make([]byte, 32))

// Compile-time interface assertions.
var (
	_ burrow.App           = (*App)(nil)
	_ burrow.Configurable  = (*App)(nil)
	_ burrow.HasMiddleware = (*App)(nil)
)

func TestAppName(t *testing.T) {
	app := &App{}
	assert.Equal(t, "session", app.Name())
}

func TestAppFlags(t *testing.T) {
	app := &App{}
	flags := app.Flags()

	names := make(map[string]bool)
	for _, f := range flags {
		names[f.Names()[0]] = true
	}

	assert.True(t, names["session-cookie-name"])
	assert.True(t, names["session-max-age"])
	assert.True(t, names["session-hash-key"])
	assert.True(t, names["session-block-key"])
}

func configuredApp(t *testing.T) *App {
	t.Helper()
	app := &App{}
	_ = app.Register(&burrow.AppConfig{})

	cmd := &cli.Command{
		Name:  "test",
		Flags: app.Flags(),
		Action: func(_ context.Context, cmd *cli.Command) error {
			return app.Configure(cmd)
		},
	}
	err := cmd.Run(t.Context(), []string{
		"test",
		"--session-hash-key", testHashKey,
	})
	require.NoError(t, err)
	return app
}

// echoWithSession creates an Echo instance with session middleware and returns
// a helper to create contexts that have the session state injected.
func echoWithSession(t *testing.T) (*echo.Echo, *App) {
	t.Helper()
	app := configuredApp(t)
	e := echo.New()
	for _, mw := range app.Middleware() {
		e.Use(mw)
	}
	return e, app
}

func TestConfigureCreatesManager(t *testing.T) {
	app := configuredApp(t)
	require.NotNil(t, app.Manager())
}

// --- Manager low-level tests ---

func TestSessionSaveAndParse(t *testing.T) {
	app := configuredApp(t)
	mgr := app.Manager()

	values := map[string]any{"user_id": int64(42), "theme": "dark"}
	cookie, err := mgr.Save(values)
	require.NoError(t, err)
	require.NotNil(t, cookie)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)

	got, err := mgr.Parse(req)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int64(42), got["user_id"])
	assert.Equal(t, "dark", got["theme"])
}

func TestSessionSaveNilValues(t *testing.T) {
	app := configuredApp(t)
	mgr := app.Manager()

	cookie, err := mgr.Save(nil)
	require.NoError(t, err)
	require.NotNil(t, cookie)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)

	got, err := mgr.Parse(req)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Empty(t, got)
}

func TestSessionParseNoCookie(t *testing.T) {
	app := configuredApp(t)
	mgr := app.Manager()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	got, err := mgr.Parse(req)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestSessionParseInvalidCookie(t *testing.T) {
	app := configuredApp(t)
	mgr := app.Manager()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "_session", Value: "garbage"})

	got, err := mgr.Parse(req)
	require.NoError(t, err)
	assert.Nil(t, got)
}

// --- Middleware tests ---

func TestMiddlewareSetsSessionValues(t *testing.T) {
	e, app := echoWithSession(t)
	mgr := app.Manager()

	cookie, err := mgr.Save(map[string]any{"user_id": int64(42)})
	require.NoError(t, err)

	var gotValues map[string]any
	e.GET("/test", func(c *echo.Context) error {
		gotValues = GetValues(c)
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, gotValues)
	assert.Equal(t, int64(42), gotValues["user_id"])
}

func TestMiddlewareNoCookie(t *testing.T) {
	e, _ := echoWithSession(t)

	var gotValues map[string]any
	e.GET("/test", func(c *echo.Context) error {
		gotValues = GetValues(c)
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Nil(t, gotValues)
}

func TestClear(t *testing.T) {
	app := configuredApp(t)
	cookie := app.Manager().Clear()
	assert.Equal(t, -1, cookie.MaxAge)
}

// --- Typed getter tests ---

func TestGetString(t *testing.T) {
	e, _ := echoWithSession(t)

	var got string
	e.GET("/test", func(c *echo.Context) error {
		// Manually inject state for getter test.
		s := getState(c)
		s.values = map[string]any{"theme": "dark", "count": int64(5)}
		got = GetString(c, "theme")
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, "dark", got)
}

func TestGetStringMissing(t *testing.T) {
	e, _ := echoWithSession(t)

	var got string
	e.GET("/test", func(c *echo.Context) error {
		got = GetString(c, "nonexistent")
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Empty(t, got)
}

func TestGetStringWrongType(t *testing.T) {
	e, _ := echoWithSession(t)

	var got string
	e.GET("/test", func(c *echo.Context) error {
		s := getState(c)
		s.values = map[string]any{"count": int64(5)}
		got = GetString(c, "count")
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Empty(t, got)
}

func TestGetInt64(t *testing.T) {
	e, _ := echoWithSession(t)

	var got int64
	e.GET("/test", func(c *echo.Context) error {
		s := getState(c)
		s.values = map[string]any{"user_id": int64(42), "name": "alice"}
		got = GetInt64(c, "user_id")
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, int64(42), got)
}

func TestGetInt64WrongType(t *testing.T) {
	e, _ := echoWithSession(t)

	var got int64
	e.GET("/test", func(c *echo.Context) error {
		s := getState(c)
		s.values = map[string]any{"name": "alice"}
		got = GetInt64(c, "name")
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, int64(0), got)
}

func TestGetValuesNoSession(t *testing.T) {
	e := echo.New() // No session middleware.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	assert.Nil(t, GetValues(c))
	assert.Empty(t, GetString(c, "anything"))
	assert.Equal(t, int64(0), GetInt64(c, "anything"))
}

// --- Context-based setter tests ---

func TestSet(t *testing.T) {
	e, _ := echoWithSession(t)

	var setErr error
	e.GET("/test", func(c *echo.Context) error {
		setErr = Set(c, "theme", "dark")
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.NoError(t, setErr)

	// Should have written a Set-Cookie header.
	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, "_session", cookies[0].Name)
}

func TestSetAddsToExistingSession(t *testing.T) {
	e, app := echoWithSession(t)
	mgr := app.Manager()

	cookie, err := mgr.Save(map[string]any{"user_id": int64(42)})
	require.NoError(t, err)

	var values map[string]any
	e.GET("/test", func(c *echo.Context) error {
		_ = Set(c, "theme", "dark")
		values = GetValues(c)
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.NotNil(t, values)
	assert.Equal(t, int64(42), values["user_id"])
	assert.Equal(t, "dark", values["theme"])
}

func TestSetWithoutMiddleware(t *testing.T) {
	e := echo.New() // No session middleware.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := Set(c, "key", "value")
	require.Error(t, err)
}

func TestDeleteKey(t *testing.T) {
	e, app := echoWithSession(t)
	mgr := app.Manager()

	cookie, err := mgr.Save(map[string]any{"user_id": int64(42), "theme": "dark"})
	require.NoError(t, err)

	var values map[string]any
	e.GET("/test", func(c *echo.Context) error {
		_ = Delete(c, "theme")
		values = GetValues(c)
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.NotNil(t, values)
	assert.Equal(t, int64(42), values["user_id"])
	_, hasTheme := values["theme"]
	assert.False(t, hasTheme)
}

func TestDeleteWithoutMiddleware(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := Delete(c, "key")
	require.Error(t, err)
}

func TestSaveContext(t *testing.T) {
	e, _ := echoWithSession(t)

	e.GET("/test", func(c *echo.Context) error {
		return Save(c, map[string]any{"user_id": int64(99)})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Verify a session cookie was written.
	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, "_session", cookies[0].Name)
}

func TestSaveContextReplacesValues(t *testing.T) {
	e, app := echoWithSession(t)
	mgr := app.Manager()

	cookie, err := mgr.Save(map[string]any{"old_key": "old_value"})
	require.NoError(t, err)

	var values map[string]any
	e.GET("/test", func(c *echo.Context) error {
		_ = Save(c, map[string]any{"new_key": "new_value"})
		values = GetValues(c)
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.NotNil(t, values)
	assert.Equal(t, "new_value", values["new_key"])
	_, hasOld := values["old_key"]
	assert.False(t, hasOld, "old values should be replaced")
}

func TestSaveContextWithoutMiddleware(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := Save(c, map[string]any{"key": "value"})
	require.Error(t, err)
}

func TestClearContext(t *testing.T) {
	e, app := echoWithSession(t)
	mgr := app.Manager()

	cookie, err := mgr.Save(map[string]any{"user_id": int64(42)})
	require.NoError(t, err)

	var values map[string]any
	e.GET("/test", func(c *echo.Context) error {
		Clear(c)
		values = GetValues(c)
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Nil(t, values) // Values should be cleared.

	// Should have written a clear cookie.
	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, -1, cookies[0].MaxAge)
}

func TestClearContextWithoutMiddleware(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Should not panic.
	assert.NotPanics(t, func() { Clear(c) })
}

// --- Roundtrip: Set then verify via Parse ---

func TestSetRoundtrip(t *testing.T) {
	e, app := echoWithSession(t)
	mgr := app.Manager()

	var responseCookies []*http.Cookie
	e.GET("/test", func(c *echo.Context) error {
		_ = Set(c, "user_id", int64(42))
		_ = Set(c, "theme", "dark")
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	responseCookies = rec.Result().Cookies()

	// Parse the last cookie (the most recent Set-Cookie wins).
	require.NotEmpty(t, responseCookies)
	lastCookie := responseCookies[len(responseCookies)-1]

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(lastCookie)

	values, err := mgr.Parse(req2)
	require.NoError(t, err)
	require.NotNil(t, values)
	assert.Equal(t, int64(42), values["user_id"])
	assert.Equal(t, "dark", values["theme"])
}

// --- Inject test helper ---

func TestInject(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	Inject(c, map[string]any{"user_id": int64(42)})

	// Getters should work.
	assert.Equal(t, int64(42), GetInt64(c, "user_id"))

	// Set should work (writes cookie).
	require.NoError(t, Set(c, "theme", "dark"))
	assert.Equal(t, "dark", GetString(c, "theme"))

	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
}

func TestInjectNilValues(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	Inject(c, nil)

	assert.Nil(t, GetValues(c))

	// Set should still work (creates the map).
	require.NoError(t, Set(c, "key", "value"))
	assert.Equal(t, "value", GetString(c, "key"))
}
