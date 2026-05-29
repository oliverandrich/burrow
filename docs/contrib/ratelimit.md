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

The rate-limit key is derived from `burrow.ClientIP(r.Context())` — the framework-wide client IP set by the server-level `--client-ip-mode` middleware. Without that middleware (downstream apps mounting ratelimit in isolation, unit tests), the key falls back to the host portion of `r.RemoteAddr` (e.g. `"1.2.3.4:5678"` → `"1.2.3.4"`).

To rate-limit by the actual end-user IP rather than the proxy's, pick the matching client-IP mode at server level. See [Client IP](../guide/client-ip.md) for the full decision tree:

```bash
# Cloudflare
--client-ip-mode=header --client-ip-header=CF-Connecting-IP

# Nginx with ngx_http_realip_module
--client-ip-mode=header --client-ip-header=X-Real-IP

# AWS ALB / dynamic proxy fleet, 2 hops
--client-ip-mode=xff-trusted-proxies --client-ip-trusted-proxies=2
```

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
| `--ratelimit-max-clients` | `RATELIMIT_MAX_CLIENTS` | `10000` | Maximum tracked clients (0 = unlimited) |

Client-IP extraction is configured at server level via `--client-ip-mode` — see [Client IP](../guide/client-ip.md).

## Graceful Shutdown

The rate limiter implements `HasShutdown` to stop the background cleanup goroutine when the server shuts down.

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()` |
| `Configurable` | Rate, burst, cleanup interval, max-clients flags |
| `HasMiddleware` | Rate limiting middleware |
| `HasShutdown` | Stops the cleanup goroutine |
