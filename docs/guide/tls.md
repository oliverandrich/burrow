# TLS

Burrow handles TLS termination directly — no reverse proxy required. It supports automatic certificates via Let's Encrypt, manual certificate files, and self-signed certificates for development.

## TLS Modes

The `--tls-mode` flag controls how TLS is configured:

| Mode | Description | Ports |
|---|---|---|
| `auto` | Smart detection (default) | Depends on host |
| `acme` | Automatic Let's Encrypt certificates | 443 + 80 (fixed) |
| `manual` | User-provided certificate and key files | User-specified |
| `selfsigned` | Auto-generated self-signed certificate | User-specified |
| `off` | Plain HTTP, no TLS | User-specified |

## Auto Mode (Default)

The default `auto` mode detects whether the server is running locally or on a public host:

- **Localhost** (`localhost`, `127.0.0.1`, `::1`, or `*.localhost`) — resolves to `off`
- **Public host** (anything else) — resolves to `acme`

This means you get the right behavior without thinking about it: plain HTTP during development, automatic HTTPS in production.

```bash
# Development — auto resolves to off (plain HTTP)
./myapp --host localhost --port 8080

# Production — auto resolves to acme (Let's Encrypt)
./myapp --host example.com
```

## ACME / Let's Encrypt

ACME mode obtains and renews TLS certificates automatically from Let's Encrypt.

```bash
./myapp --host example.com --tls-mode acme --tls-email admin@example.com
```

=== "CLI Flags"

    ```bash
    ./myapp --host example.com --tls-mode acme --tls-email admin@example.com
    ```

=== "Environment Variables"

    ```bash
    HOST=example.com TLS_MODE=acme TLS_EMAIL=admin@example.com ./myapp
    ```

=== "TOML Config"

    ```toml
    [server]
    host = "example.com"

    [tls]
    mode = "acme"
    email = "admin@example.com"
    ```

**How it works:**

1. The server binds to port **443** (HTTPS) and port **80** (HTTP challenge + redirect)
2. On first request, it obtains a certificate from Let's Encrypt via the HTTP-01 challenge
3. Certificates are cached in `--tls-cert-dir` (default: `./data/certs`) and renewed automatically
4. All HTTP traffic on port 80 is redirected to HTTPS

**Requirements:**

- The `--host` flag must be set to a public domain name
- Port 80 and 443 must be reachable from the internet
- The `--port` flag must **not** be set — ACME always uses the standard ports
- An email address (`--tls-email`) is required for Let's Encrypt registration

!!! warning "Let's Encrypt rate limits"
    Let's Encrypt enforces [rate limits](https://letsencrypt.org/docs/rate-limits/): 50 certificates per domain per week, and 5 failed validation attempts per hour. During development, test with a self-signed certificate (`--tls-mode selfsigned`) instead of ACME to avoid hitting these limits.

!!! important
    Make sure your DNS points to the server **before** starting with ACME mode. Let's Encrypt validates domain ownership via HTTP, so the server must be reachable at the specified host.

## Manual Certificates

Use your own certificate and key files:

```bash
./myapp --tls-mode manual --tls-cert-file /path/to/cert.pem --tls-key-file /path/to/key.pem
```

Both `--tls-cert-file` and `--tls-key-file` are required. The files must be PEM-encoded.

This mode is useful when you obtain certificates through a separate process (e.g. your organization's PKI, or a wildcard certificate).

## Self-Signed Certificates

For development and testing, Burrow can generate a self-signed certificate automatically:

```bash
./myapp --tls-mode selfsigned --port 8443
```

On first startup, an ECDSA P-256 certificate is generated and stored in `--tls-cert-dir`:

- `selfsigned-cert.pem`
- `selfsigned-key.pem`

The certificate is valid for 365 days and includes SANs for the specified host, `localhost`, `127.0.0.1`, and `::1`. On subsequent starts, the existing certificate is reused.

!!! note
    Browsers will show a security warning for self-signed certificates. This mode is intended for development only.

## Disabling TLS

To run plain HTTP without any TLS:

```bash
./myapp --tls-mode off --port 8080
```

This is appropriate for local development or when running behind a load balancer that handles TLS termination.

## Graceful Restart and TLS

When using [graceful restart](deployment.md#graceful-restart-sighup) (SIGHUP), all listeners — including both the HTTPS and HTTP challenge ports in ACME mode — are passed to the new child process via file descriptor inheritance. This means:

- No dropped connections during restart
- Certificate renewal continues without interruption
- The PID file is updated to point to the new process

## Configuration Reference

| Flag | Env Var | TOML | Default | Description |
|---|---|---|---|---|
| `--tls-mode` | `TLS_MODE` | `tls.mode` | `auto` | TLS mode: `auto`, `acme`, `selfsigned`, `manual`, `off` |
| `--tls-cert-dir` | `TLS_CERT_DIR` | `tls.cert_dir` | `./data/certs` | Directory for cached/generated certificates |
| `--tls-email` | `TLS_EMAIL` | `tls.email` | — | Email for ACME registration (required for `acme` mode) |
| `--tls-cert-file` | `TLS_CERT_FILE` | `tls.cert_file` | — | Path to PEM certificate file (required for `manual` mode) |
| `--tls-key-file` | `TLS_KEY_FILE` | `tls.key_file` | — | Path to PEM private key file (required for `manual` mode) |

All TLS connections use TLS 1.2 as the minimum version.
