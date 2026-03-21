// Package sse provides a Server-Sent Events (SSE) contrib app for burrow.
//
// It implements an in-memory pub/sub broker for topic-based event distribution.
// Clients connect via SSE endpoints and receive events in real time.
// The package integrates naturally with htmx's SSE extension (hx-ext="sse",
// sse-connect, sse-swap).
//
// # Usage
//
// Register the SSE app and use the broker to publish events:
//
//	sseApp := sse.New()
//	srv := burrow.NewServer(sseApp, /* other apps */)
//
// In your app's Routes, create SSE endpoints:
//
//	func (a *MyApp) Routes(r chi.Router) {
//	    r.Get("/events", sse.Handler(a.broker, "notifications"))
//	}
//
// Publish events from request handlers:
//
//	a.broker.Publish("notifications", sse.Event{Data: "<p>New!</p>"})
package sse

import "context"

type ctxKeyBroker struct{}

// WithBroker stores an EventBroker in the context.
func WithBroker(ctx context.Context, b *EventBroker) context.Context {
	return context.WithValue(ctx, ctxKeyBroker{}, b)
}

// Broker retrieves the EventBroker from the context.
// Returns nil if no broker is present.
func Broker(ctx context.Context) *EventBroker {
	b, _ := ctx.Value(ctxKeyBroker{}).(*EventBroker)
	return b
}
