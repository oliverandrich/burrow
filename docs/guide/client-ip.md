# Client IP

burrow exposes the client IP framework-wide so contribs (rate limiting today; request logging, geofencing, abuse detection tomorrow) read a single source instead of each rolling their own header parser.

!!! info "Upgrading from `--ratelimit-trust-proxy`?"
    The flag is removed. Jump to [Migration](#migration-from-the-old-ratelimit-trust-proxy-flag) — the right replacement depends on your proxy (Cloudflare, Nginx, AWS, etc.), so don't just guess `X-Real-IP`.

## The trust problem

There is no safe default for "what's the client IP." `r.RemoteAddr` is the TCP peer, which is the client if you're directly on the internet and the reverse proxy otherwise. Headers like `X-Forwarded-For` are useful behind known proxies and trivially spoofable everywhere else. Quoting [chi v5.3.0's release notes](https://github.com/go-chi/chi/releases/tag/v5.3.0):

> There is no safe default — the user must pick their trust source explicitly.

burrow takes that stance literally: `--client-ip-mode` is a required choice, and every non-default mode demands its companion flag.

## Picking a mode

| Deployment | Mode | Companion |
|---|---|---|
| No proxy / server directly on the internet | `remote-addr` (default) | — |
| Cloudflare | `header` | `--client-ip-header=CF-Connecting-IP` |
| Nginx with `ngx_http_realip_module` | `header` | `--client-ip-header=X-Real-IP` |
| Apache with `mod_remoteip` | `header` | `--client-ip-header=X-Client-IP` |
| Known proxy CIDRs (AWS CloudFront, etc.) | `xff-trusted-cidrs` | `--client-ip-trusted-cidrs=13.32.0.0/15,52.46.0.0/18` |
| Dynamic proxy fleet (autoscaling, ephemeral) | `xff-trusted-proxies` | `--client-ip-trusted-proxies=2` |

`header` mode is safe only when your proxy **unconditionally overwrites** the named header on every request. `True-Client-IP`, `X-Azure-ClientIP`, and `Fastly-Client-IP` look similar but pass client-supplied values through by default in those products — don't use them unless your edge strips the inbound value first.

!!! danger "Trust boundary for header-mode CDNs (Cloudflare, etc.)"
    `--client-ip-header=CF-Connecting-IP` is correct only when burrow accepts connections **exclusively** from Cloudflare's edge — either via a firewall restricting inbound traffic to [Cloudflare's IP ranges](https://www.cloudflare.com/ips/), via [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/) (`cloudflared`), or behind a private network. If your origin is reachable on the public internet, anyone hitting it directly can forge `CF-Connecting-IP` and burrow has no way to tell. The same logic applies to any header-mode CDN.

`xff-trusted-proxies` is brittle: if you add or remove a proxy hop and forget to update the count, you'll silently start trusting an attacker-supplied IP. Prefer `xff-trusted-cidrs` whenever you can enumerate the proxy IP ranges.

## Reading the result

In handlers and middleware:

```go
import "github.com/oliverandrich/burrow"

func handler(w http.ResponseWriter, r *http.Request) error {
    ip := burrow.ClientIP(r.Context())   // string; "" if no middleware ran
    addr := burrow.ClientIPAddr(r.Context()) // netip.Addr; check addr.IsValid()
    // log, rate-limit, geo-lookup, etc.
}
```

`burrow.ClientIP` and `burrow.ClientIPAddr` re-export chi's `GetClientIP` / `GetClientIPAddr`, so the framework's source of client IP is the same one chi's middleware populates.

## Configuration

All four flags accept env-var and TOML sources (consistent with every other burrow flag):

```toml
# burrow.toml
[server.client_ip]
mode = "header"
header = "X-Real-IP"

# Or, for a dynamic AWS ALB fleet:
# mode = "xff-trusted-proxies"
# trusted_proxies = 2
```

Boot fails fast on misconfiguration: setting a companion flag without the matching mode (or selecting a mode without its companion) returns an error from `boot` rather than silently picking an unintended source.

## Migration from the old ratelimit-trust-proxy flag

`--ratelimit-trust-proxy` (removed) was equivalent to:

```
--client-ip-mode=header --client-ip-header=X-Real-IP
```

Update your config; `contrib/ratelimit` now derives its rate-limit key from `burrow.ClientIP` regardless of which mode you pick.
