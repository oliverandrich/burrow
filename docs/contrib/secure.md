# Security Headers

Security response headers middleware using [unrolled/secure](https://github.com/unrolled/secure).

**Package:** `github.com/oliverandrich/burrow/contrib/secure`

**Depends on:** none

## Setup

```go
srv := burrow.NewServer(
    secure.New(),
    // ... other apps
)
```

With options:

```go
secure.New(
    secure.WithContentSecurityPolicy("default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'"),
    secure.WithPermissionsPolicy("camera=(), microphone=()"),
    secure.WithCrossOriginOpenerPolicy("same-origin"),
)
```

## Default Headers

The following headers are set on every response without any configuration:

### X-Content-Type-Options: nosniff

Prevents browsers from guessing (MIME-sniffing) the content type of a response. Without this header, a browser could interpret a file served as `text/plain` as JavaScript and execute it if the content looks like code.

### X-Frame-Options: DENY

Completely prevents the page from being embedded in an `<iframe>`. Protects against clickjacking attacks, where an attacker overlays your app invisibly on top of another page so users unknowingly click buttons in your application.

### Referrer-Policy: strict-origin-when-cross-origin

Controls what information is sent in the `Referer` header to other sites. For same-origin requests, the full path is included. For cross-origin requests, only the origin is sent (e.g., `https://example.com` instead of `https://example.com/admin/users/42`). Prevents sensitive URL paths from leaking to external services.

### Strict-Transport-Security (HSTS)

Auto-detected from `BaseURL`. When the URL starts with `https://`, HSTS is enabled with `max-age=63072000; includeSubDomains; preload` (2 years). This tells browsers to always use HTTPS for your domain, preventing protocol downgrade attacks. For plain HTTP, `IsDevelopment` mode is enabled automatically, which disables HSTS, SSL redirect, and host checking.

### Not Set by Default

The following headers have no safe universal default — every application has different requirements:

- **Content-Security-Policy** — See [Recommended CSP](#recommended-csp) below
- **Permissions-Policy**
- **Cross-Origin-Opener-Policy**

## Constructor Options

Constructor options take precedence over CLI flags (set at code level, immutable at deploy time).

### WithContentSecurityPolicy(csp)

Sets the `Content-Security-Policy` header. Defines a whitelist of where the browser may load resources from (scripts, styles, images, fonts, etc.). Extremely effective against XSS, but every app has different external resources, so no default is set. See [Recommended CSP](#recommended-csp) for ready-to-use policies.

### WithPermissionsPolicy(pp)

Sets the `Permissions-Policy` header. Controls which browser APIs the page may use. For example, `"camera=(), microphone=(), geolocation=()"` disables camera, microphone, and geolocation entirely. Useful for reducing attack surface, but depends on which APIs your app actually needs.

### WithCrossOriginOpenerPolicy(coop)

Sets the `Cross-Origin-Opener-Policy` header. Isolates the browser window from cross-origin popups. `"same-origin"` prevents a window opened via `window.open()` from accessing your `window` object. Protects against side-channel attacks (Spectre etc.), but can break OAuth popups or payment flows.

### WithAllowedHosts(hosts...)

Validates the `Host` header of every request against a whitelist. Protects against Host header injection, where an attacker sends a manipulated Host header to redirect password reset links or other generated URLs to their own domain.

### WithSSLRedirect(bool)

Redirects HTTP requests to HTTPS via 301. Opt-in because many deployments use a reverse proxy (nginx, Caddy) that already handles this.

### WithSSLProxyHeaders(map)

Tells the middleware which proxy headers indicate that the original request was HTTPS. For example, `map[string]string{"X-Forwarded-Proto": "https"}` for deployments behind a load balancer that terminates TLS.

### WithDevelopment(bool)

Overrides auto-detection. When `true`, HSTS, SSL redirect, and host checking are disabled — useful for local development. Automatically enabled for HTTP base URLs, but can be forced explicitly.

## Configuration

| Flag | Env Var | TOML Key | Default | Description |
|------|---------|----------|---------|-------------|
| `--secure-csp` | `SECURE_CSP` | `secure.csp` | `""` | Content-Security-Policy |
| `--secure-permissions-policy` | `SECURE_PERMISSIONS_POLICY` | `secure.permissions_policy` | `""` | Permissions-Policy |
| `--secure-coop` | `SECURE_COOP` | `secure.coop` | `""` | Cross-Origin-Opener-Policy |
| `--secure-allowed-hosts` | `SECURE_ALLOWED_HOSTS` | `secure.allowed_hosts` | `""` | Comma-separated allowed hosts |
| `--secure-ssl-redirect` | `SECURE_SSL_REDIRECT` | `secure.ssl_redirect` | `false` | Redirect HTTP to HTTPS |
| `--secure-development` | `SECURE_DEVELOPMENT` | `secure.development` | auto | Force development mode |

## Recommended CSP

CSP is not set by default because no single policy works for all applications. However, Burrow's built-in contrib apps (Bootstrap, HTMX, Auth, Admin) use inline `<script>` tags and inline `style` attributes, so a working CSP requires `'unsafe-inline'` for both.

### Typical Burrow + Bootstrap + HTMX Setup

```go
secure.New(
    secure.WithContentSecurityPolicy(
        "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'",
    ),
)
```

This allows:

- **`default-src 'self'`** — Only load resources from your own origin (images, fonts, media, etc.)
- **`style-src 'self' 'unsafe-inline'`** — Own stylesheets plus inline styles (needed by admin layout sidebar, Bootstrap Icons SVGs)
- **`script-src 'self' 'unsafe-inline'`** — Own scripts plus inline scripts (needed by theme switcher, admin history sync, auth WebAuthn flows)

### Stricter Setup (No Inline Content)

If you avoid inline scripts and styles in your own templates (keeping them only in contrib apps), you can tighten the policy for connect, images, and fonts:

```go
secure.New(
    secure.WithContentSecurityPolicy(
        "default-src 'none'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self'; font-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'self'; frame-ancestors 'none'",
    ),
)
```

### With External Resources

If your app loads resources from external CDNs or APIs, add them explicitly:

```go
secure.New(
    secure.WithContentSecurityPolicy(
        "default-src 'self'; style-src 'self' 'unsafe-inline' fonts.googleapis.com; font-src fonts.gstatic.com; script-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' api.example.com",
    ),
)
```

!!! note
    Burrow's contrib apps currently require `'unsafe-inline'` for both `style-src` and `script-src`. A future version may extract inline content into separate files to allow stricter CSP policies.

## Development Mode

Development mode disables HSTS, SSL redirect, and allowed host checking. It is auto-enabled when the `BaseURL` uses HTTP. Override with `WithDevelopment(true/false)` or `--secure-development`.

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `Configurable` | CSP, permissions policy, COOP, allowed hosts, SSL redirect, development mode |
| `HasMiddleware` | Security headers middleware |
