package sse

import (
	"log/slog"
	"net/http"
	"time"
)

const keepaliveInterval = 30 * time.Second

// ContextHandler returns an http.HandlerFunc that serves an SSE stream,
// retrieving the broker from the request context (injected by the SSE
// middleware). This is the recommended way to create SSE endpoints.
//
//	r.Get("/events", sse.ContextHandler("notifications", "updates"))
func ContextHandler(topics ...string) http.HandlerFunc {
	return ContextHandlerFunc(func(_ *http.Request) []string {
		return topics
	})
}

// ContextHandlerFunc is like ContextHandler but determines topics dynamically
// per request.
//
//	r.Get("/events/{room}", sse.ContextHandlerFunc(func(r *http.Request) []string {
//	    return []string{"room:" + chi.URLParam(r, "room")}
//	}))
func ContextHandlerFunc(topicsFn func(r *http.Request) []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b := Broker(r.Context())
		if b == nil {
			http.Error(w, "sse: no broker in context", http.StatusInternalServerError)
			return
		}
		HandlerFunc(b, topicsFn).ServeHTTP(w, r)
	}
}

// Handler returns an http.HandlerFunc that serves an SSE stream.
// It subscribes the client to the given topics and streams events
// until the client disconnects or the broker shuts down.
//
// This returns http.HandlerFunc (not burrow.HandlerFunc) because SSE
// connections are long-lived streams that manage their own error handling.
// Do NOT wrap with burrow.Handle().
//
//	r.Get("/events", sse.Handler(broker, "notifications", "updates"))
func Handler(b *EventBroker, topics ...string) http.HandlerFunc {
	return HandlerFunc(b, func(_ *http.Request) []string {
		return topics
	})
}

// HandlerFunc is like Handler but determines topics dynamically per request.
//
//	r.Get("/events/{room}", sse.HandlerFunc(broker, func(r *http.Request) []string {
//	    return []string{"room:" + chi.URLParam(r, "room")}
//	}))
func HandlerFunc(b *EventBroker, topicsFn func(r *http.Request) []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		topics := topicsFn(r)
		c := b.Subscribe(topics...)
		defer b.Unsubscribe(c)

		ticker := time.NewTicker(keepaliveInterval)
		defer ticker.Stop()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.Done():
				return
			case event, ok := <-c.Events():
				if !ok {
					return
				}
				if err := event.Format(w); err != nil {
					slog.Debug("sse: write event", "error", err)
					return
				}
				flusher.Flush()
			case <-ticker.C:
				if _, err := w.Write([]byte(":keepalive\n\n")); err != nil {
					slog.Debug("sse: write keepalive", "error", err)
					return
				}
				flusher.Flush()
			}
		}
	}
}
