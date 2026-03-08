package session

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Middleware tests (via context getters) ---

func TestMiddlewareSetsSessionValues(t *testing.T) {
	r, app := routerWithSession(t)
	mgr := app.Manager()

	cookie, err := mgr.Save(map[string]any{"user_id": int64(42)})
	require.NoError(t, err)

	var gotValues map[string]any
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		gotValues = GetValues(r)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, gotValues)
	assert.Equal(t, int64(42), gotValues["user_id"])
}

func TestMiddlewareNoCookie(t *testing.T) {
	r, _ := routerWithSession(t)

	var gotValues map[string]any
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		gotValues = GetValues(r)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Nil(t, gotValues)
}

// --- Typed getter tests ---

func TestGetString(t *testing.T) {
	r, _ := routerWithSession(t)

	var got string
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		s := getState(r)
		s.values = map[string]any{"theme": "dark", "count": int64(5)}
		got = GetString(r, "theme")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, "dark", got)
}

func TestGetStringMissing(t *testing.T) {
	r, _ := routerWithSession(t)

	var got string
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		got = GetString(r, "nonexistent")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Empty(t, got)
}

func TestGetStringWrongType(t *testing.T) {
	r, _ := routerWithSession(t)

	var got string
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		s := getState(r)
		s.values = map[string]any{"count": int64(5)}
		got = GetString(r, "count")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Empty(t, got)
}

func TestGetInt64(t *testing.T) {
	r, _ := routerWithSession(t)

	var got int64
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		s := getState(r)
		s.values = map[string]any{"user_id": int64(42), "name": "alice"}
		got = GetInt64(r, "user_id")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, int64(42), got)
}

func TestGetInt64WrongType(t *testing.T) {
	r, _ := routerWithSession(t)

	var got int64
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		s := getState(r)
		s.values = map[string]any{"name": "alice"}
		got = GetInt64(r, "name")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, int64(0), got)
}

func TestGetValuesReturnsCopy(t *testing.T) {
	r, app := routerWithSession(t)
	mgr := app.Manager()

	cookie, err := mgr.Save(map[string]any{"user_id": int64(42)})
	require.NoError(t, err)

	var original, afterMutation map[string]any
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		original = GetValues(r)
		// Mutate the returned map — this must NOT affect the internal state.
		original["injected"] = "evil"
		afterMutation = GetValues(r)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, afterMutation)
	_, hasInjected := afterMutation["injected"]
	assert.False(t, hasInjected, "mutation of returned map should not affect internal state")
}

func TestGetValuesNoSession(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	assert.Nil(t, GetValues(req))
	assert.Empty(t, GetString(req, "anything"))
	assert.Equal(t, int64(0), GetInt64(req, "anything"))
}

// --- Context-based setter tests ---

func TestSet(t *testing.T) {
	r, _ := routerWithSession(t)

	var setErr error
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		setErr = Set(w, r, "theme", "dark")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.NoError(t, setErr)

	// Should have written a Set-Cookie header.
	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, "_session", cookies[0].Name)
}

func TestSetAddsToExistingSession(t *testing.T) {
	r, app := routerWithSession(t)
	mgr := app.Manager()

	cookie, err := mgr.Save(map[string]any{"user_id": int64(42)})
	require.NoError(t, err)

	var values map[string]any
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		_ = Set(w, r, "theme", "dark")
		values = GetValues(r)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.NotNil(t, values)
	assert.Equal(t, int64(42), values["user_id"])
	assert.Equal(t, "dark", values["theme"])
}

func TestSetWithoutMiddleware(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	err := Set(rec, req, "key", "value")
	require.Error(t, err)
}

func TestDeleteKey(t *testing.T) {
	r, app := routerWithSession(t)
	mgr := app.Manager()

	cookie, err := mgr.Save(map[string]any{"user_id": int64(42), "theme": "dark"})
	require.NoError(t, err)

	var values map[string]any
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		_ = Delete(w, r, "theme")
		values = GetValues(r)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.NotNil(t, values)
	assert.Equal(t, int64(42), values["user_id"])
	_, hasTheme := values["theme"]
	assert.False(t, hasTheme)
}

func TestDeleteWithoutMiddleware(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	err := Delete(rec, req, "key")
	require.Error(t, err)
}

func TestSaveContext(t *testing.T) {
	r, _ := routerWithSession(t)

	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		_ = Save(w, r, map[string]any{"user_id": int64(99)})
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Verify a session cookie was written.
	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, "_session", cookies[0].Name)
}

func TestSaveContextReplacesValues(t *testing.T) {
	r, app := routerWithSession(t)
	mgr := app.Manager()

	cookie, err := mgr.Save(map[string]any{"old_key": "old_value"})
	require.NoError(t, err)

	var values map[string]any
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		_ = Save(w, r, map[string]any{"new_key": "new_value"})
		values = GetValues(r)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.NotNil(t, values)
	assert.Equal(t, "new_value", values["new_key"])
	_, hasOld := values["old_key"]
	assert.False(t, hasOld, "old values should be replaced")
}

func TestSaveContextWithoutMiddleware(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	err := Save(rec, req, map[string]any{"key": "value"})
	require.Error(t, err)
}

func TestClearContext(t *testing.T) {
	r, app := routerWithSession(t)
	mgr := app.Manager()

	cookie, err := mgr.Save(map[string]any{"user_id": int64(42)})
	require.NoError(t, err)

	var values map[string]any
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		Clear(w, r)
		values = GetValues(r)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Nil(t, values) // Values should be cleared.

	// Should have written a clear cookie.
	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, -1, cookies[0].MaxAge)
}

func TestClearContextWithoutMiddleware(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// Should not panic.
	assert.NotPanics(t, func() { Clear(rec, req) })
}

// --- Roundtrip: Set then verify via Parse ---

func TestSetRoundtrip(t *testing.T) {
	r, app := routerWithSession(t)
	mgr := app.Manager()

	var responseCookies []*http.Cookie
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		_ = Set(w, r, "user_id", int64(42))
		_ = Set(w, r, "theme", "dark")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	responseCookies = rec.Result().Cookies()

	// Parse the last cookie (the most recent Set-Cookie wins).
	require.NotEmpty(t, responseCookies)
	lastCookie := responseCookies[len(responseCookies)-1]

	req2 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req2.AddCookie(lastCookie)

	values, err := mgr.Parse(req2)
	require.NoError(t, err)
	require.NotNil(t, values)
	assert.Equal(t, int64(42), values["user_id"])
	assert.Equal(t, "dark", values["theme"])
}

// --- Inject test helper ---

func TestInject(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	req = Inject(req, map[string]any{"user_id": int64(42)})

	// Getters should work.
	assert.Equal(t, int64(42), GetInt64(req, "user_id"))

	// Set should work (writes cookie).
	require.NoError(t, Set(rec, req, "theme", "dark"))
	assert.Equal(t, "dark", GetString(req, "theme"))

	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
}

func TestInjectNilValues(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	req = Inject(req, nil)

	assert.Nil(t, GetValues(req))

	// Set should still work (creates the map).
	require.NoError(t, Set(rec, req, "key", "value"))
	assert.Equal(t, "value", GetString(req, "key"))
}
