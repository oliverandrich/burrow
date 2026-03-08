# Healthcheck

A minimal health check endpoint that reports server and database status.

**Package:** `github.com/oliverandrich/burrow/contrib/healthcheck`

**Depends on:** none

## Setup

```go
srv := burrow.NewServer(
    healthcheck.New(),
    // ... other apps
)
```

## Endpoint

### GET /healthz

Returns the server and database status as JSON.

**Healthy response** (200 OK):

```json
{
    "status": "ok",
    "database": "ok"
}
```

**Unhealthy response** (503 Service Unavailable):

```json
{
    "status": "ok",
    "database": "connection refused"
}
```

The endpoint pings the database to check connectivity. If the ping fails, it returns 503 with the error message in the `database` field.

## Usage

```bash
curl http://localhost:8080/healthz
```

Use this endpoint for:

- Load balancer health checks
- Kubernetes liveness/readiness probes
- Monitoring systems

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `HasRoutes` | `/healthz` endpoint |
