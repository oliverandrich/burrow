# Configuration Reference

Complete list of all configuration flags, environment variables, and TOML keys.

## Core Flags

### Server

| Flag | Env Var | TOML Key | Default | Description |
|------|---------|----------|---------|-------------|
| `--host` | `HOST` | `server.host` | `localhost` | Host to bind to |
| `--port` | `PORT` | `server.port` | `8080` | Port to listen on |
| `--base-url` | `BASE_URL` | `server.base_url` | (auto-resolved) | Base URL for the application |
| `--max-body-size` | `MAX_BODY_SIZE` | `server.max_body_size` | `1` | Maximum request body size in MB |
| `--shutdown-timeout` | `SHUTDOWN_TIMEOUT` | `server.shutdown_timeout` | `10` | Graceful shutdown timeout in seconds |
| `--pid-file` | `PID_FILE` | `server.pid_file` | (none) | Path to PID file (for systemd/supervisor) |

### Database

| Flag | Env Var | TOML Key | Default | Description |
|------|---------|----------|---------|-------------|
| `--database-dsn` | `DATABASE_DSN` | `database.dsn` | `app.db` | Database DSN |

### TLS

| Flag | Env Var | TOML Key | Default | Description |
|------|---------|----------|---------|-------------|
| `--tls-mode` | `TLS_MODE` | `tls.mode` | `auto` | TLS mode: `auto`, `acme`, `selfsigned`, `manual`, `off` |
| `--tls-cert-dir` | `TLS_CERT_DIR` | `tls.cert_dir` | `./data/certs` | Directory for auto-generated certificates |
| `--tls-email` | `TLS_EMAIL` | `tls.email` | (none) | Email for ACME/Let's Encrypt registration |
| `--tls-cert-file` | `TLS_CERT_FILE` | `tls.cert_file` | (none) | Path to TLS certificate file (manual mode) |
| `--tls-key-file` | `TLS_KEY_FILE` | `tls.key_file` | (none) | Path to TLS private key file (manual mode) |

### i18n

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--i18n-default-language` | `I18N_DEFAULT_LANGUAGE` | `en` | Default language |
| `--i18n-supported-languages` | `I18N_SUPPORTED_LANGUAGES` | `en,de` | Comma-separated supported languages |

## Contrib App Flags

### Session

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--session-cookie-name` | `SESSION_COOKIE_NAME` | `_session` | Session cookie name |
| `--session-max-age` | `SESSION_MAX_AGE` | `604800` | Session max age in seconds (7 days) |
| `--session-hash-key` | `SESSION_HASH_KEY` | (auto-generated) | 32-byte hex key for cookie signing |
| `--session-block-key` | `SESSION_BLOCK_KEY` | (none) | 32-byte hex key for cookie encryption |

### CSRF

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--csrf-key` | `CSRF_KEY` | (auto-generated) | 32-byte hex key for token signing |

### Auth

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--auth-login-redirect` | `AUTH_LOGIN_REDIRECT` | `/` | Redirect target after login |
| `--auth-logout-redirect` | `AUTH_LOGOUT_REDIRECT` | `/auth/login` | Redirect target after logout |
| `--auth-use-email` | `AUTH_USE_EMAIL` | `false` | Use email instead of username |
| `--auth-require-verification` | `AUTH_REQUIRE_VERIFICATION` | `false` | Require email verification before login |
| `--auth-invite-only` | `AUTH_INVITE_ONLY` | `false` | Require invite to register |
| `--webauthn-rp-id` | `WEBAUTHN_RP_ID` | `localhost` | WebAuthn Relying Party ID (domain name) |
| `--webauthn-rp-display-name` | `WEBAUTHN_RP_DISPLAY_NAME` | `Web App` | WebAuthn RP display name |
| `--webauthn-rp-origin` | `WEBAUTHN_RP_ORIGIN` | (base URL) | WebAuthn RP origin |

### Jobs

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--jobs-workers` | `JOBS_WORKERS` | `2` | Number of concurrent worker goroutines |
| `--jobs-poll-interval` | `JOBS_POLL_INTERVAL` | `1s` | Interval between queue polls |
| `--jobs-retry-base-delay` | `JOBS_RETRY_BASE_DELAY` | `30s` | Base delay for exponential retry backoff |

### Uploads

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--upload-dir` | `UPLOAD_DIR` | `./uploads` | Directory for uploaded files |
| `--upload-url-prefix` | `UPLOAD_URL_PREFIX` | `/uploads` | URL prefix for serving uploaded files |
| `--upload-allowed-types` | `UPLOAD_ALLOWED_TYPES` | (all) | Comma-separated list of allowed MIME types |

### Rate Limit

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--ratelimit-rate` | `RATELIMIT_RATE` | `10` | Requests per second (token refill rate) |
| `--ratelimit-burst` | `RATELIMIT_BURST` | `20` | Maximum burst size (bucket capacity) |
| `--ratelimit-cleanup-interval` | `RATELIMIT_CLEANUP_INTERVAL` | `1m` | Interval for sweeping expired entries |
| `--ratelimit-trust-proxy` | `RATELIMIT_TRUST_PROXY` | `false` | Use X-Real-IP header for client IP |
| `--ratelimit-max-clients` | `RATELIMIT_MAX_CLIENTS` | `10000` | Maximum tracked clients (0 = unlimited) |

### Secure

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--secure-csp` | `SECURE_CSP` | (none) | Content-Security-Policy header value |
| `--secure-permissions-policy` | `SECURE_PERMISSIONS_POLICY` | (none) | Permissions-Policy header value |
| `--secure-coop` | `SECURE_COOP` | (none) | Cross-Origin-Opener-Policy header value |
| `--secure-allowed-hosts` | `SECURE_ALLOWED_HOSTS` | (none) | Comma-separated list of allowed hostnames |
| `--secure-ssl-redirect` | `SECURE_SSL_REDIRECT` | `false` | Redirect HTTP requests to HTTPS |
| `--secure-development` | `SECURE_DEVELOPMENT` | `false` | Disable HSTS, SSL redirect, host checks |

### SMTP Mail

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--smtp-host` | `SMTP_HOST` | `localhost` | SMTP server host |
| `--smtp-port` | `SMTP_PORT` | `587` | SMTP server port |
| `--smtp-username` | `SMTP_USERNAME` | (none) | SMTP username |
| `--smtp-password` | `SMTP_PASSWORD` | (none) | SMTP password |
| `--smtp-from` | `SMTP_FROM` | `noreply@localhost` | Sender email address |

## TOML Config Example

```toml
[server]
host = "0.0.0.0"
port = 8080
base_url = "https://myapp.example.com"
max_body_size = 2
pid_file = "/run/myapp/server.pid"

[database]
dsn = "./data/production.db"

[tls]
mode = "acme"
email = "admin@example.com"
cert_dir = "./data/certs"
```

!!! note
    All flags support CLI, environment variable, and TOML sourcing. The TOML key is shown in each contrib app's flag definition (e.g., `jobs.workers`, `ratelimit.rate`).
