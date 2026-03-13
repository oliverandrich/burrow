# Deployment

Burrow compiles to a single static binary with an embedded SQLite database. There is no app server, no external database, and no runtime dependency to install. Deployment is: copy the binary, run it.

## Deployment Options

| Option | Best For |
|--------|----------|
| [Bare metal / VPS](#bare-metal-vps) | Simple standalone deployment, single server |
| [systemd service](#systemd) | Production Linux servers with automatic restarts and log management |
| [Docker](#docker) | Container orchestration, CI/CD pipelines |

All three options support graceful shutdown. Bare metal and systemd also support [zero-downtime restarts](#graceful-restart-sighup) via `SIGHUP`.

For TLS, the server handles ACME certificates, manual certs, or self-signed certs natively — no reverse proxy needed. See the [TLS guide](tls.md) for details.

## Secret Keys

Burrow auto-generates random keys for session signing and CSRF protection when none are configured. This is convenient during development, but **sessions and CSRF tokens will not survive a server restart** because the keys are only held in memory.

For production, always set these environment variables (or their CLI/TOML equivalents):

```bash
# Generate keys (32 bytes, hex-encoded)
export SESSION_HASH_KEY=$(openssl rand -hex 32)
export SESSION_BLOCK_KEY=$(openssl rand -hex 32)  # optional, enables session encryption
export CSRF_KEY=$(openssl rand -hex 32)
```

| Variable | Purpose | If missing |
|----------|---------|------------|
| `SESSION_HASH_KEY` | Signs session cookies (HMAC) | Auto-generated; sessions lost on restart |
| `SESSION_BLOCK_KEY` | Encrypts session cookies (AES) | Sessions are signed but not encrypted |
| `CSRF_KEY` | Signs CSRF tokens | Auto-generated; tokens invalid on restart |

!!! warning
    Without persistent keys, every server restart silently logs out all users and invalidates all pending forms. The server logs a warning when keys are auto-generated — treat this as a deployment misconfiguration in production.

## Bare Metal / VPS

For a simple standalone deployment without a process manager:

```bash
# Build
go build -o server ./cmd/server

# Run with PID file for restart support
./server --pid-file ./server.pid &

# Graceful restart (e.g. after deploying a new binary)
kill -HUP $(cat ./server.pid)

# Graceful shutdown
kill $(cat ./server.pid)
```

For production use, a process manager like systemd is recommended to handle automatic restarts on failure and log management.

## systemd

Use `Type=forking` so systemd tracks the new process after a graceful restart. The `PIDFile` directive tells systemd where to find the current PID.

```ini
[Unit]
Description=My Burrow App
After=network.target

[Service]
Type=forking
PIDFile=/run/myapp/server.pid
Environment=SESSION_HASH_KEY=<your-key>
Environment=CSRF_KEY=<your-key>
ExecStart=/usr/local/bin/server --pid-file /run/myapp/server.pid
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5

# Recommended hardening
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/myapp /run/myapp

[Install]
WantedBy=multi-user.target
```

With this setup:

- `systemctl start myapp` — starts the server
- `systemctl reload myapp` — sends SIGHUP, triggers zero-downtime restart
- `systemctl stop myapp` — sends SIGTERM, triggers graceful shutdown
- `systemctl restart myapp` — stop + start (brief downtime)

!!! tip
    Always prefer `systemctl reload` over `systemctl restart` for configuration changes and binary upgrades.

## Docker

In a Docker container, the server runs as PID 1. Graceful restart is automatically disabled because the parent exiting would cause the container runtime to stop the container.

The server still supports graceful **shutdown** — when Docker sends `SIGTERM` (via `docker stop`), in-flight requests are drained before exit.

```dockerfile
FROM golang:1.25 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=builder /server /server
EXPOSE 8080
# Set SESSION_HASH_KEY and CSRF_KEY via docker run -e or compose environment
ENTRYPOINT ["/server"]
```

For zero-downtime deployments with Docker, use an orchestrator-level rolling restart (e.g. `docker compose up -d --force-recreate`) rather than in-process restarts.

## Graceful Restart (SIGHUP)

On Linux and macOS, the server supports zero-downtime restarts via `SIGHUP`. When a `SIGHUP` signal is received, the server:

1. Spawns a new child process that inherits the listener file descriptors
2. The child starts accepting connections
3. The parent stops accepting new connections and drains in-flight requests
4. The parent exits

This allows binary upgrades and configuration changes without dropping any connections.

```bash
# Trigger a graceful restart
kill -HUP $(cat /run/myapp/server.pid)
```

!!! note
    The parent process exits after the child takes over. If you start the server from a terminal, the shell prompt returns while the child continues running in the background. Use a PID file or process manager (systemd, supervisor) to manage the server lifecycle.

### PID File

Use `--pid-file` to write the server's PID to a file. The PID file is updated on each restart so it always points to the current active process.

```bash
./server --pid-file /run/myapp/server.pid
```

### When Graceful Restart Is Disabled

The server automatically falls back to simple mode (no graceful restart, only graceful shutdown) in these situations:

- **Windows** — `SIGHUP` and file descriptor inheritance are not available
- **PID 1 (containers)** — the parent exiting would stop the container (see [Docker](#docker) above)
- **Concurrent upgrader** — if tableflip cannot initialize (e.g. in tests)

In simple mode, `SIGINT` and `SIGTERM` still trigger a graceful shutdown that drains in-flight requests.

## TLS

See the [TLS guide](tls.md) for full details on ACME, self-signed, manual, and auto TLS modes.
