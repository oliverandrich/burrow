# Rate Limiting

Per-client rate limiting middleware using a token bucket algorithm.

**Package:** `github.com/oliverandrich/burrow/contrib/ratelimit`

**Depends on:** none

## Setup

```go
srv := burrow.NewServer(
    ratelimit.New(),
    // ... other apps
)
```

With options:

```go
ratelimit.New(
    ratelimit.WithKeyFunc(func(r *http.Request) string {
        // Custom key extraction (e.g., API key, user ID)
        return r.Header.Get("X-API-Key")
    }),
    ratelimit.WithOnLimited(func(w http.ResponseWriter, r *http.Request) {
        // Custom response for rate-limited requests
        http.Error(w, "Slow down!", http.StatusTooManyRequests)
    }),
)
```

## How It Works

The rate limiter uses a **token bucket** algorithm from `golang.org/x/time/rate`:

1. Each client (identified by IP or custom key) gets a bucket with `burst` tokens
2. Tokens are refilled at `rate` tokens per second
3. Each request consumes one token
4. When the bucket is empty, the request is rejected with HTTP 429 and a `Retry-After` header

Idle client entries are automatically cleaned up at the configured interval.

## Client Identification

By default, the client IP is extracted from `RemoteAddr`. When `--ratelimit-trust-proxy` is enabled, the middleware uses the `X-Real-IP` header instead.

Configure your reverse proxy to set `X-Real-IP` to the actual client IP:

```nginx
# nginx
proxy_set_header X-Real-IP $remote_addr;
```

```
# Caddy (sets X-Real-IP automatically)
```

!!! warning
    Without `--ratelimit-trust-proxy`, all traffic behind a reverse proxy appears to come from the proxy's IP — effectively rate limiting all clients as one. With the flag enabled but no `X-Real-IP` header set, the middleware falls back to `RemoteAddr`.

Override with `WithKeyFunc()` for custom identification (e.g., by API key or authenticated user).

## Context Helpers

When a request is rate-limited, the `Retry-After` duration is available in the context:

```go
import "github.com/oliverandrich/burrow/contrib/ratelimit"

retryAfter := ratelimit.RetryAfter(r.Context())
```

This is useful in custom `OnLimited` handlers.

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--ratelimit-rate` | `RATELIMIT_RATE` | `10` | Requests per second (token refill rate) |
| `--ratelimit-burst` | `RATELIMIT_BURST` | `20` | Maximum burst size (bucket capacity) |
| `--ratelimit-cleanup-interval` | `RATELIMIT_CLEANUP_INTERVAL` | `1m` | Interval for sweeping expired entries |
| `--ratelimit-trust-proxy` | `RATELIMIT_TRUST_PROXY` | `false` | Use `X-Real-IP` header for client IP |
| `--ratelimit-max-clients` | `RATELIMIT_MAX_CLIENTS` | `10000` | Maximum tracked clients (0 = unlimited) |

## Graceful Shutdown

The rate limiter implements `HasShutdown` to stop the background cleanup goroutine when the server shuts down.

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `Configurable` | Rate, burst, cleanup interval, and trust-proxy flags |
| `HasMiddleware` | Rate limiting middleware |
| `HasShutdown` | Stops the cleanup goroutine |
