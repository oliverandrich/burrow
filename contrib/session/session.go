// Package session provides cookie-based session management as a burrow contrib app.
package session

import (
	"crypto/rand"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v5"
)

func init() {
	gob.Register(map[string]any{})
	gob.Register(int64(0))
	gob.Register(time.Time{})
}

// cookiePayload is the internal structure encoded in the cookie.
// ExpiresAt is managed by the session Manager; Values holds the app-provided data.
type cookiePayload struct {
	ExpiresAt time.Time
	Values    map[string]any
}

const storeKey = "session:state"

// state holds the session state for the current request.
type state struct {
	manager *Manager
	values  map[string]any
}

// save encodes the current values into a cookie and sets it on the response.
func (s *state) save(c *echo.Context) error {
	cookie, err := s.manager.Save(s.values)
	if err != nil {
		return err
	}
	c.SetCookie(cookie)
	return nil
}

// getState returns the session state from the context, or nil if no middleware is active.
func getState(c *echo.Context) *state {
	if s, ok := c.Get(storeKey).(*state); ok {
		return s
	}
	return nil
}

// errNoMiddleware is returned when session functions are called without the session middleware.
var errNoMiddleware = errors.New("session: no session middleware")

// --- Context-based getters ---

// GetValues retrieves all session values from the Echo context.
// Returns nil if no session is active.
func GetValues(c *echo.Context) map[string]any {
	s := getState(c)
	if s == nil {
		return nil
	}
	return s.values
}

// GetString retrieves a string value from the session.
func GetString(c *echo.Context, key string) string {
	values := GetValues(c)
	if values == nil {
		return ""
	}
	v, _ := values[key].(string)
	return v
}

// GetInt64 retrieves an int64 value from the session.
func GetInt64(c *echo.Context, key string) int64 {
	values := GetValues(c)
	if values == nil {
		return 0
	}
	n, _ := values[key].(int64)
	return n
}

// --- Context-based setters ---

// Set sets a single value in the session and immediately writes the cookie.
func Set(c *echo.Context, key string, value any) error {
	s := getState(c)
	if s == nil {
		return errNoMiddleware
	}
	if s.values == nil {
		s.values = make(map[string]any)
	}
	s.values[key] = value
	return s.save(c)
}

// Delete removes a key from the session and immediately writes the cookie.
func Delete(c *echo.Context, key string) error {
	s := getState(c)
	if s == nil {
		return errNoMiddleware
	}
	delete(s.values, key)
	return s.save(c)
}

// Save replaces all session values and immediately writes the cookie.
// Use this for operations like login where you want a fresh session.
func Save(c *echo.Context, values map[string]any) error {
	s := getState(c)
	if s == nil {
		return errNoMiddleware
	}
	s.values = values
	return s.save(c)
}

// Clear clears the session and writes a deletion cookie.
func Clear(c *echo.Context) {
	s := getState(c)
	if s == nil {
		return
	}
	s.values = nil
	c.SetCookie(s.manager.Clear())
}

// Inject sets up session state in the context without the full middleware.
// This is intended for use in tests by other packages.
func Inject(c *echo.Context, values map[string]any) {
	sc := securecookie.New(make([]byte, 32), nil)
	sc.MaxAge(3600)
	mgr := &Manager{sc: sc, cookieName: "_session", maxAge: 3600}
	c.Set(storeKey, &state{manager: mgr, values: values})
}

// --- Manager ---

// Manager handles session cookie creation and parsing.
type Manager struct {
	sc         *securecookie.SecureCookie
	cookieName string
	maxAge     int
	secure     bool
}

// Save creates a new session cookie with the given values.
func (m *Manager) Save(values map[string]any) (*http.Cookie, error) {
	payload := cookiePayload{
		ExpiresAt: time.Now().Add(time.Duration(m.maxAge) * time.Second),
		Values:    values,
	}

	encoded, err := m.sc.Encode(m.cookieName, payload)
	if err != nil {
		return nil, err
	}

	return &http.Cookie{
		Name:     m.cookieName,
		Value:    encoded,
		Path:     "/",
		MaxAge:   m.maxAge,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	}, nil
}

// Parse parses the session cookie from the request.
// Returns nil, nil if no valid session is present.
func (m *Manager) Parse(r *http.Request) (map[string]any, error) {
	cookie, err := r.Cookie(m.cookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return nil, nil
		}
		return nil, err
	}

	var payload cookiePayload
	if err := m.sc.Decode(m.cookieName, cookie.Value, &payload); err != nil {
		slog.Debug("invalid session cookie", "error", err)
		return nil, nil //nolint:nilerr // Invalid cookie is treated as no session
	}

	if time.Now().After(payload.ExpiresAt) {
		return nil, nil
	}

	if payload.Values == nil {
		return map[string]any{}, nil
	}

	return payload.Values, nil
}

// Clear returns a cookie that clears the session.
func (m *Manager) Clear() *http.Cookie {
	return &http.Cookie{
		Name:     m.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// --- Key utilities ---

func resolveKey(keyHex, keyType string) ([]byte, error) {
	if keyHex != "" {
		return decodeKey(keyHex, keyType)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, errors.New("failed to generate session " + keyType + " key")
	}
	slog.Warn("No session "+keyType+" key configured, using random key (sessions will not persist across restarts)",
		"generated_key", hex.EncodeToString(key),
	)
	return key, nil
}

func decodeKey(keyHex, keyType string) ([]byte, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, errors.New("invalid session " + keyType + " key: must be hex encoded")
	}
	if len(key) != 32 {
		return nil, errors.New("invalid session " + keyType + " key: must be 32 bytes")
	}
	return key, nil
}
