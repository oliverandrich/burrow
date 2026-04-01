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
func (a *App) CreateNote(w http.ResponseWriter, r *http.Request) error {
    note, err := a.repo.Create(r.Context(), input)
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

## Accessing the Broker Outside HTTP Handlers

The context-based `sse.Broker(ctx)` helper requires a request context with injected middleware. For background jobs, CLI commands, or other non-HTTP code, use `BrokerFromRegistry` instead:

```go
func (h *NotificationJobHandler) Run(ctx context.Context) error {
    broker := sse.BrokerFromRegistry(h.registry)
    if broker == nil {
        return nil // SSE not available, skip notification
    }

    broker.Publish("notifications", sse.Event{
        Data: "<p>Background task completed!</p>",
    })
    return nil
}
```

`BrokerFromRegistry` returns `nil` if the SSE app is not registered or has not been configured yet — always check for `nil` before using the broker.

## Rendering Templates in Background Jobs

In HTTP handlers, `burrow.Render` has access to the full request context (locale, user, CSRF token, etc.). Background jobs don't have a request, but you can still render templates using `burrow.RenderFragment` with a manually enriched context.

**Step 1:** Store the server's template executor when setting up your job handler. Since templates are built during `Server.Run()`, obtain the executor from an HTTP handler or a startup hook that runs after boot — not from `Register` or `Configure`:

```go
type ArticleNotifier struct {
    exec     burrow.TemplateExecutor
    broker   *sse.EventBroker
    registry *burrow.Registry
}

func NewArticleNotifier(srv *burrow.Server, registry *burrow.Registry) *ArticleNotifier {
    return &ArticleNotifier{
        exec:     srv.TemplateExecutor(),
        broker:   sse.BrokerFromRegistry(registry),
        registry: registry,
    }
}
```

**Step 2:** In the job, build a context with the values your template needs and call `RenderFragment`:

```go
func (n *ArticleNotifier) Notify(ctx context.Context, article *Article, locale string) error {
    if n.broker == nil {
        return nil
    }

    // Enrich context with template executor and locale
    ctx = burrow.WithTemplateExecutor(ctx, n.exec)
    ctx = i18n.WithLocale(ctx, locale)

    html, err := burrow.RenderFragment(ctx, "articles/list_item", map[string]any{
        "Article": article,
    })
    if err != nil {
        return err
    }

    n.broker.Publish("articles", sse.Event{Data: string(html)})
    return nil
}
```

!!! tip
    Only enrich the context with values your template actually uses. SSE fragments typically need the **locale** for translations, but not CSRF tokens, flash messages, or navigation items — those are HTTP-only concerns.

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
| `burrow.App` | Required: `Name()` |
| `burrow.Configurable` | `sse-buffer-size` flag |
| `burrow.HasMiddleware` | Injects broker into request context |
| `burrow.HasShutdown` | Closes broker and disconnects all clients |
