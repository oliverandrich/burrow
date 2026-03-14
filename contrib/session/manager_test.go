package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/securecookie"
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.AddCookie(cookie)

	got, err := mgr.Parse(req)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Empty(t, got)
}

func TestSessionParseNoCookie(t *testing.T) {
	app := configuredApp(t)
	mgr := app.Manager()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	got, err := mgr.Parse(req)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestSessionParseInvalidCookie(t *testing.T) {
	app := configuredApp(t)
	mgr := app.Manager()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "_session", Value: "garbage"})

	got, err := mgr.Parse(req)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestSessionParseExpiredCookie(t *testing.T) {
	// Create a manager with a very short max age so the cookie expires immediately.
	sc := securecookie.New(make([]byte, 32), nil)
	sc.MaxAge(1)
	mgr := &Manager{sc: sc, cookieName: "_session", maxAge: 1}

	// Save a session with an expiry in the past by manually encoding.
	payload := cookiePayload{
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		Values:    map[string]any{"user_id": int64(42)},
	}
	encoded, err := sc.Encode("_session", payload)
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "_session", Value: encoded})

	got, err := mgr.Parse(req)
	require.NoError(t, err)
	assert.Nil(t, got, "expired session should return nil values")
}

func TestSessionParseCorruptedCookieVariants(t *testing.T) {
	app := configuredApp(t)
	mgr := app.Manager()

	tests := []struct {
		name  string
		value string
	}{
		{"empty value", ""},
		{"random garbage", "not-a-valid-cookie-at-all"},
		{"base64 but invalid", "dGhpcyBpcyBub3QgYSB2YWxpZCBjb29raWU="},
		{"truncated", "MTcxNTAwMDAwMHxE"},
		{"special characters", "!@#$%^&*()"},
		{"very long garbage", string(make([]byte, 4096))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			req.AddCookie(&http.Cookie{Name: "_session", Value: tt.value})

			got, err := mgr.Parse(req)
			require.NoError(t, err, "corrupted cookie should not return an error")
			assert.Nil(t, got, "corrupted cookie should return nil values")
		})
	}
}

func TestSessionParseWrongKey(t *testing.T) {
	// Save a cookie with one key, try to parse with a different key.
	sc1 := securecookie.New([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), nil)
	sc1.MaxAge(3600)
	mgr1 := &Manager{sc: sc1, cookieName: "_session", maxAge: 3600}

	cookie, err := mgr1.Save(map[string]any{"secret": "data"})
	require.NoError(t, err)

	sc2 := securecookie.New([]byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), nil)
	sc2.MaxAge(3600)
	mgr2 := &Manager{sc: sc2, cookieName: "_session", maxAge: 3600}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.AddCookie(cookie)

	got, err := mgr2.Parse(req)
	require.NoError(t, err, "wrong key should be treated as invalid, not error")
	assert.Nil(t, got)
}

func TestClear(t *testing.T) {
	app := configuredApp(t)
	cookie := app.Manager().Clear()
	assert.Equal(t, -1, cookie.MaxAge)
}
