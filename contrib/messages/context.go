// Package messages provides flash message support as a burrow contrib app.
//
// Messages are stored in a request-scoped store and in the session cookie.
// Handlers add messages via [Add] (or the convenience helpers [AddInfo],
// [AddSuccess], [AddWarning], [AddError]), and templates read them via [Get].
//
// The unified API works for both redirect flows (message persisted in
// session, read on next request) and same-request rendering (HTMX partial
// responses). When [Get] is called in the same request as [Add], it returns
// the messages from the in-memory store and clears the session cookie so the
// message is not displayed again after a redirect.
package messages

import (
	"context"
	"net/http"

	"codeberg.org/oliverandrich/burrow/contrib/session"
)

// messageStore is a mutable, request-scoped container for flash messages.
// It holds references to w and r so that Get can clear the session cookie
// when messages are consumed during the same request.
type messageStore struct { //nolint:govet // fieldalignment: readability over optimization
	messages []Message
	w        http.ResponseWriter
	r        *http.Request
}

// ctxKeyStore is the context key for the mutable message store.
type ctxKeyStore struct{}

// withStore stores the mutable message store in the context.
func withStore(ctx context.Context, store *messageStore) context.Context {
	return context.WithValue(ctx, ctxKeyStore{}, store)
}

// getStore retrieves the mutable message store from the context.
func getStore(ctx context.Context) *messageStore {
	s, _ := ctx.Value(ctxKeyStore{}).(*messageStore)
	return s
}

// ctxKeyMessages is the context key for immutable flash messages (used by Inject).
type ctxKeyMessages struct{}

// Get returns flash messages for the current request. It first checks the
// mutable store (populated by the middleware and Add calls), then falls back
// to the immutable context value (set by [Inject] for tests).
//
// When reading from the store, messages are consumed: subsequent calls
// return nil. The session cookie is also cleared to prevent double-display
// after a redirect.
func Get(ctx context.Context) []Message {
	if store := getStore(ctx); store != nil {
		msgs := store.messages
		store.messages = nil
		if len(msgs) > 0 {
			_ = session.Delete(store.w, store.r, sessionKey)
		}
		return msgs
	}
	// Fallback: immutable context (tests using Inject).
	msgs, _ := ctx.Value(ctxKeyMessages{}).([]Message)
	return msgs
}

// Inject stores messages in the context for use in tests. It allows other
// packages to set up message state without running the full middleware.
func Inject(ctx context.Context, msgs []Message) context.Context {
	return context.WithValue(ctx, ctxKeyMessages{}, msgs)
}
