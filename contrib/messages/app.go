package messages

import (
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/session"
)

// New creates a new messages app.
func New() *App { return &App{} }

// App implements flash message support as a burrow contrib app.
// It requires the session app to be registered first.
type App struct{}

func (a *App) Name() string { return "messages" }

func (a *App) Register(_ *burrow.AppConfig) error { return nil }

func (a *App) Dependencies() []string { return []string{"session"} }

func (a *App) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{flashMiddleware}
}

// flashMiddleware creates a mutable message store seeded from any messages
// persisted in the session. The store is placed in the request context so
// that [Add] and [Get] can operate on it during the request lifetime.
func flashMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var initial []Message
		values := session.GetValues(r)
		if raw, ok := values[sessionKey]; ok {
			if msgs, ok := raw.([]Message); ok && len(msgs) > 0 {
				initial = msgs
				_ = session.Delete(w, r, sessionKey)
			}
		}

		store := &messageStore{messages: initial, w: w, r: r}
		r = r.WithContext(withStore(r.Context(), store))
		next.ServeHTTP(w, r)
	})
}
