# Healthcheck

Liveness and readiness probes for monitoring and orchestration.

**Package:** `github.com/oliverandrich/burrow/contrib/healthcheck`

**Depends on:** none

## Setup

```go
srv := burrow.NewServer(
    healthcheck.New(),
    // ... other apps
)
```

## Endpoints

### GET /healthz/live

Liveness probe. Always returns 200 OK — confirms the process is running.

```json
{
    "status": "ok"
}
```

### GET /healthz/ready

Readiness probe. Pings the database and runs `ReadinessCheck` on all registered apps that implement [`ReadinessChecker`](../reference/interfaces.md#readinesschecker). Returns 200 if everything passes, 503 if any check fails.

**All ready** (200 OK):

```json
{
    "status": "ok",
    "database": "ok",
    "checks": {
        "cache": "ok"
    }
}
```

**Not ready** (503 Service Unavailable):

```json
{
    "status": "unavailable",
    "database": "ok",
    "checks": {
        "cache": "ok",
        "queue": "queue down"
    }
}
```

Apps that do not implement `ReadinessChecker` are not included in `checks`.

## Usage

```bash
# Kubernetes liveness probe
curl http://localhost:8080/healthz/live

# Kubernetes readiness probe
curl http://localhost:8080/healthz/ready
```

Use these endpoints for:

- Load balancer health checks (`/healthz/ready`)
- Kubernetes liveness probes (`/healthz/live`)
- Kubernetes readiness probes (`/healthz/ready`)
- Monitoring systems

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `HasRoutes` | `/healthz/live`, `/healthz/ready` endpoints |
