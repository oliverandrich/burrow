# SSE (Server-Sent Events)

Provides an in-memory pub/sub broker for [Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events). Clients connect via SSE endpoints and receive real-time updates pushed from the server. Integrates naturally with [htmx's SSE extension](https://htmx.org/extensions/sse/).

**Package:** `github.com/oliverandrich/burrow/contrib/sse`

## Quick Start

### 1. Register the app

```go
srv := burrow.NewServer(
    sse.New(),
    // ... your other apps
)
```

The SSE app creates a broker automatically and injects it into every request via middleware. You don't need to wire anything manually.

### 2. Add an SSE endpoint

In your app's `Routes()`, register an SSE endpoint:

```go
func (a *App) Routes(r chi.Router) {
    r.Get("/events", sse.ContextHandler("notifications"))
}
```

That's it — clients connecting to `/events` will receive all events published to the `"notifications"` topic.

### 3. Publish events from a handler

Anywhere in a request handler, grab the broker from the context and publish:

```go
func (h *Handlers) CreateNote(w http.ResponseWriter, r *http.Request) error {
    note, err := h.repo.Create(r.Context(), input)
    if err != nil {
        return err
    }

    // Push update to all connected SSE clients
    sse.Broker(r.Context()).Publish("notifications", sse.Event{
        Data: fmt.Sprintf(`<li>%s</li>`, note.Title),
    })

    return burrow.Render(w, r, http.StatusOK, "notes/created", data)
}
```

### 4. Receive events in the browser

With htmx's SSE extension, no JavaScript is needed:

```html
<div hx-ext="sse" sse-connect="/events" sse-swap="notifications">
    Waiting for updates...
</div>
```

When the server publishes to `"notifications"`, htmx automatically swaps the content of this `<div>` with the event's `Data` field.

## Dynamic Topics

Use `ContextHandlerFunc` when topics depend on the request (e.g., per-room or per-resource channels):

```go
func (a *App) Routes(r chi.Router) {
    r.Get("/projects/{id}/events", sse.ContextHandlerFunc(func(r *http.Request) []string {
        return []string{"project:" + chi.URLParam(r, "id")}
    }))
}
```

Then publish to a specific project:

```go
sse.Broker(r.Context()).Publish("project:42", sse.Event{
    Data: `<span class="badge bg-success">Updated</span>`,
})
```

Only clients watching project 42 receive the event.

## Multiple Topics per Endpoint

A single SSE endpoint can subscribe to multiple topics:

```go
r.Get("/events", sse.ContextHandler("notifications", "status"))
```

```html
<div hx-ext="sse" sse-connect="/events">
    <div sse-swap="notifications">No notifications</div>
    <div sse-swap="status">Status: unknown</div>
</div>
```

Each `sse-swap` target only receives events matching its topic name.

## Event Format

The `Event` struct maps to the [SSE wire format](https://html.spec.whatwg.org/multipage/server-sent-events.html):

```go
sse.Event{
    Type:  "notifications",   // SSE "event:" field — maps to htmx sse-swap
    Data:  "<p>New item!</p>", // SSE "data:" field — multi-line handled automatically
    ID:    "42",               // SSE "id:" field (optional)
    Retry: 5000,               // SSE "retry:" field in ms (optional)
}
```

When publishing, if `Type` is empty it is automatically set to the topic name:

```go
// Event.Type will be "alerts" — no need to set it explicitly
broker.Publish("alerts", sse.Event{Data: "fire!"})
```

## Configuration

| Flag | Env | TOML | Default | Description |
|------|-----|------|---------|-------------|
| `sse-buffer-size` | `SSE_BUFFER_SIZE` | `sse.buffer_size` | `16` | Per-client event buffer capacity |

When a client's buffer is full, new events are silently dropped for that client. The publisher is never blocked.

## Advanced: Explicit Broker

For cases where you need a standalone broker (e.g., in tests or with multiple brokers), you can create one explicitly and pass it to `Handler` / `HandlerFunc` directly:

```go
broker := sse.NewEventBroker(16)
defer broker.Close()

r.Get("/events", sse.Handler(broker, "topic"))
```

Most applications should use `ContextHandler` instead — it picks up the broker from middleware automatically.

## Keepalive

The handler sends a `:keepalive` comment every 30 seconds to prevent reverse proxies from closing idle connections.

## Graceful Shutdown

On server shutdown, the broker closes all client connections. SSE handlers detect the closed connection and return, allowing the HTTP server's graceful shutdown to complete.

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `burrow.Configurable` | `sse-buffer-size` flag |
| `burrow.HasMiddleware` | Injects broker into request context |
| `burrow.HasShutdown` | Closes broker and disconnects all clients |
