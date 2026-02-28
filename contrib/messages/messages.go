package messages

import (
	"encoding/gob"
	"net/http"

	"codeberg.org/oliverandrich/burrow/contrib/session"
)

func init() {
	gob.Register([]Message{})
}

const sessionKey = "_messages"

// Level represents the severity of a flash message.
type Level string

const (
	Info    Level = "info"
	Success Level = "success"
	Warning Level = "warning"
	Error   Level = "error"
)

// Message is a single flash message with a severity level.
type Message struct {
	Level Level
	Text  string
}

// Add appends a flash message to the request-scoped store and the session.
// The store makes the message available via [Get] in the same request (for
// HTMX partial responses). The session persists the message for redirect
// flows where [Get] is called on the next request.
func Add(w http.ResponseWriter, r *http.Request, level Level, text string) error {
	msg := Message{Level: level, Text: text}

	// Write to mutable store for same-request rendering.
	if store := getStore(r.Context()); store != nil {
		store.messages = append(store.messages, msg)
	}

	// Write to session for redirect persistence (skip if no session middleware).
	values := session.GetValues(r)
	if values == nil {
		return nil
	}
	var msgs []Message //nolint:prealloc // length unknown before reading session
	if existing, ok := values[sessionKey]; ok {
		msgs, _ = existing.([]Message)
	}
	msgs = append(msgs, msg)
	return session.Set(w, r, sessionKey, msgs)
}

// AddInfo is a convenience helper that adds an info-level message.
func AddInfo(w http.ResponseWriter, r *http.Request, text string) error {
	return Add(w, r, Info, text)
}

// AddSuccess is a convenience helper that adds a success-level message.
func AddSuccess(w http.ResponseWriter, r *http.Request, text string) error {
	return Add(w, r, Success, text)
}

// AddWarning is a convenience helper that adds a warning-level message.
func AddWarning(w http.ResponseWriter, r *http.Request, text string) error {
	return Add(w, r, Warning, text)
}

// AddError is a convenience helper that adds an error-level message.
func AddError(w http.ResponseWriter, r *http.Request, text string) error {
	return Add(w, r, Error, text)
}
