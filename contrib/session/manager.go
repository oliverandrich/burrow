package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
)

// cookiePayload is the internal structure encoded in the cookie.
// ExpiresAt is managed by the session Manager; Values holds the app-provided data.
type cookiePayload struct {
	ExpiresAt time.Time
	Values    map[string]any
}

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
