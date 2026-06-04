# Reverse Proxy

Behind a TLS-terminating reverse proxy (nginx, Caddy, Cloudflare, ngrok) the proxy speaks HTTPS to the browser and plain HTTP to your app. Go then sees `r.TLS == nil`, so without help burrow treats every request as HTTP. That breaks anything that depends on the public scheme:

- **CSRF**: a browser `Origin: https://host` mismatches the computed `http://host` → `403 "origin invalid"` on every state-changing POST.
- **Session cookies**: the `Secure` attribute is wrong.
- **HSTS**: never emitted.

burrow fixes this by trusting the proxy's `X-Forwarded-Proto` to set the per-request scheme — but only from a proxy you trust, gated on the direct TCP peer.

This assumes your proxy sets that header (nginx: `proxy_set_header X-Forwarded-Proto $scheme;`; Caddy, Cloudflare, and ngrok send it automatically).

## Zero-config default

The default `--forwarded-mode=private` trusts `X-Forwarded-Proto` from **loopback and private (RFC1918 / IPv6 ULA) peers**. A same-host nginx or Caddy reaches your app over `127.0.0.1`, and an ngrok or local-tunnel agent likewise connects over loopback, so the common "app behind a proxy on the same box" setup (including shared hosting like Uberspace) works with **no configuration** — just set `--base-url` to your public `https://…` URL. A directly-served app on a VPS that terminates its own TLS is also unaffected — it already sees `r.TLS`, and public clients aren't in the trusted range.

!!! danger "Public and CGNAT peers are never trusted by default"
    `X-Forwarded-Proto` is trivially spoofable. The default range deliberately **excludes** public addresses and CGNAT (`100.64.0.0/10` — the shared range used by Uberspace and similar hosts), because on shared infrastructure a neighbour could forge the header and flip your `Secure` cookies / CSRF scheme. If your proxy reaches the app from a public or CGNAT address, name it explicitly with `--forwarded-mode=trusted-cidrs`.

## Modes

| Deployment | Mode | Companion |
|---|---|---|
| Same-host proxy (nginx/Caddy over loopback), ngrok / local-tunnel agent, private-network/Docker-bridge proxy | `private` (default) | — |
| Same-host proxy, paranoid (trust loopback only) | `loopback` | — |
| Proxy on a public/CGNAT address, a CDN edge range, or a known single-tenant subnet | `trusted-cidrs` | `--forwarded-trusted-cidrs=…` |
| No proxy, or you want forwarded headers ignored entirely | `off` | — |

```bash
# A Cloudflare / specific-subnet proxy on a non-private address:
myapp --forwarded-mode=trusted-cidrs --forwarded-trusted-cidrs=203.0.113.0/24
```

Set `--base-url` to your public `https://…` URL regardless of mode — it drives absolute-URL generation, WebAuthn, and email links, and makes the `Secure`/HSTS defaults correct on their own.

## What it does (and doesn't) touch

When the peer is trusted, the middleware records the scheme in the request context and installs a sentinel `r.TLS` (for HTTPS), so `burrow.RequestIsHTTPS(r)` reports HTTPS. It leaves `r.URL` untouched — a server request's URL is origin-form (path only), so the public scheme/host belong in `--base-url`, not on `r.URL`. It is **upgrade-only**: an `X-Forwarded-Proto: http` never clears a genuine TLS connection.

- **CSRF** consults `burrow.RequestIsHTTPS(r)` for its origin check — the `403` disappears.
- **Sessions** mark the cookie `Secure` for forwarded-HTTPS requests (never downgrading an https base URL).
- **HSTS** (`contrib/secure`) is emitted for forwarded-HTTPS requests **only when you explicitly set `--forwarded-mode`** — the zero-config default is not treated as a "this is production behind a proxy" signal, so local http development keeps its dev-mode defaults.
- **WebAuthn RP origin** is unchanged: it stays anchored to `--base-url`, never derived from a header.

!!! warning "`X-Forwarded-Host` is opt-in"
    `--forwarded-trust-host` additionally applies `X-Forwarded-Host` to `r.Host`. It is off by default: an attacker-controlled Host enables cache poisoning and poisoned absolute URLs. Enable it only behind a proxy that overwrites the header.

## Relationship to `--client-ip-mode`

[Client IP](client-ip.md) and forwarded headers are **independent trust decisions** with separate flags. One answers "who is the client" (from `X-Forwarded-For`); the other answers "what scheme did the browser use" (from `X-Forwarded-Proto`). Configure each for your proxy; burrow never infers one from the other.

!!! note "For contrib authors"
    The forwarded middleware sets `r.TLS` to an empty sentinel `&tls.ConnectionState{}`. Check `r.TLS != nil` (or call `burrow.RequestIsHTTPS(r)`) to test for HTTPS — never read certificate fields off it, as they are absent for a proxied request.
