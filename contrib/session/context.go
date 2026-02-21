// Package session provides cookie-based session management as a burrow contrib app.
package session

import (
	"context"
	"encoding/gob"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
)

func init() {
	gob.Register(map[string]any{})
	gob.Register(int64(0))
	gob.Register(time.Time{})
	gob.Register([]string{})
}

// ctxKeySession is the context key for session state.
type ctxKeySession struct{}

// state holds the session state for the current request.
type state struct {
	manager *Manager
	values  map[string]any
}

// save encodes the current values into a cookie and sets it on the response.
func (s *state) save(w http.ResponseWriter) error {
	cookie, err := s.manager.Save(s.values)
	if err != nil {
		return err
	}
	http.SetCookie(w, cookie)
	return nil
}

// getState returns the session state from the request context, or nil if no middleware is active.
func getState(r *http.Request) *state {
	if s, ok := r.Context().Value(ctxKeySession{}).(*state); ok {
		return s
	}
	return nil
}

// errNoMiddleware is returned when session functions are called without the session middleware.
var errNoMiddleware = errors.New("session: no session middleware")

// --- Context-based getters ---

// GetValues retrieves all session values from the request.
// Returns nil if no session is active.
func GetValues(r *http.Request) map[string]any {
	s := getState(r)
	if s == nil {
		return nil
	}
	return s.values
}

// GetString retrieves a string value from the session.
func GetString(r *http.Request, key string) string {
	values := GetValues(r)
	if values == nil {
		return ""
	}
	v, _ := values[key].(string)
	return v
}

// GetInt64 retrieves an int64 value from the session.
func GetInt64(r *http.Request, key string) int64 {
	values := GetValues(r)
	if values == nil {
		return 0
	}
	n, _ := values[key].(int64)
	return n
}

// --- Context-based setters ---

// Set sets a single value in the session and immediately writes the cookie.
func Set(w http.ResponseWriter, r *http.Request, key string, value any) error {
	s := getState(r)
	if s == nil {
		return errNoMiddleware
	}
	if s.values == nil {
		s.values = make(map[string]any)
	}
	s.values[key] = value
	return s.save(w)
}

// Delete removes a key from the session and immediately writes the cookie.
func Delete(w http.ResponseWriter, r *http.Request, key string) error {
	s := getState(r)
	if s == nil {
		return errNoMiddleware
	}
	delete(s.values, key)
	return s.save(w)
}

// Save replaces all session values and immediately writes the cookie.
// Use this for operations like login where you want a fresh session.
func Save(w http.ResponseWriter, r *http.Request, values map[string]any) error {
	s := getState(r)
	if s == nil {
		return errNoMiddleware
	}
	s.values = values
	return s.save(w)
}

// Clear clears the session and writes a deletion cookie.
func Clear(w http.ResponseWriter, r *http.Request) {
	s := getState(r)
	if s == nil {
		return
	}
	s.values = nil
	http.SetCookie(w, s.manager.Clear())
}

// Inject sets up session state in the request context without the full middleware.
// This is intended for use in tests by other packages. It returns a new request
// with the session state injected.
func Inject(r *http.Request, values map[string]any) *http.Request {
	sc := securecookie.New(make([]byte, 32), nil)
	sc.MaxAge(3600)
	mgr := &Manager{sc: sc, cookieName: "_session", maxAge: 3600}
	s := &state{manager: mgr, values: values}
	ctx := r.Context()
	ctx = context.WithValue(ctx, ctxKeySession{}, s)
	return r.WithContext(ctx)
}
