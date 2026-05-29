package server

import (
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/oliverandrich/burrow/app"
)

// clientIPMiddleware returns the chi middleware that matches the configured
// mode. The caller is expected to have invoked [app.Config.ValidateClientIP]
// first; an unrecognised mode falls back to remote-addr.
//
// Mode-to-middleware mapping (see docs/guide/client-ip.md):
//
//   - remote-addr           → [chimw.ClientIPFromRemoteAddr] — TCP peer IP only,
//     correct when this server is directly on the public internet.
//   - header                → [chimw.ClientIPFromHeader](cfg.Header) — trust the
//     single-IP header the reverse proxy unconditionally overwrites.
//   - xff-trusted-proxies   → [chimw.ClientIPFromXFFTrustedProxies](cfg.TrustedProxies) —
//     walk back the configured number of hops in X-Forwarded-For.
//   - xff-trusted-cidrs     → [chimw.ClientIPFromXFF](cfg.TrustedCIDRs...) — walk
//     back X-Forwarded-For skipping IPs in the trusted CIDR set.
func clientIPMiddleware(cfg app.ClientIPConfig) func(http.Handler) http.Handler {
	switch cfg.Mode {
	case "header":
		return chimw.ClientIPFromHeader(cfg.Header)
	case "xff-trusted-proxies":
		return chimw.ClientIPFromXFFTrustedProxies(cfg.TrustedProxies)
	case "xff-trusted-cidrs":
		return chimw.ClientIPFromXFF(cfg.TrustedCIDRs...)
	default:
		return chimw.ClientIPFromRemoteAddr
	}
}
