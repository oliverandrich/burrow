package session

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestClear(t *testing.T) {
	app := configuredApp(t)
	cookie := app.Manager().Clear()
	assert.Equal(t, -1, cookie.MaxAge)
}
