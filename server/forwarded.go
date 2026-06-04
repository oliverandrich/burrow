package server

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/oliverandrich/burrow/app"
)

// loopbackPrefixes covers same-host reverse proxies (nginx/Caddy that reach the
// app over 127.0.0.1 / ::1, e.g. Uberspace).
var loopbackPrefixes = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("::1/128"),
}

// privatePrefixes is the default trust set: loopback plus RFC1918 and IPv6 ULA,
// covering same-host and private-network/Docker-bridge proxies. Public and CGNAT
// (100.64.0.0/10 — shared hosting) peers are deliberately excluded. Declared
// after loopbackPrefixes so the append picks up the initialised value.
var privatePrefixes = append([]netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("fc00::/7"),
}, loopbackPrefixes...)

// resolveTrustedPrefixes maps the configured mode to its trusted-peer prefix
// set. "off" (and an unparseable trusted-cidrs list) yields nil, which the
// middleware treats as a no-op passthrough.
func resolveTrustedPrefixes(cfg app.ForwardedConfig) []netip.Prefix {
	switch cfg.Mode {
	case "", "private":
		return privatePrefixes
	case "loopback":
		return loopbackPrefixes
	case "trusted-cidrs":
		return parsePrefixes(cfg.TrustedCIDRs)
	default: // "off" or unknown (validated at boot)
		return nil
	}
}

// parsePrefixes parses CIDR strings into masked prefixes, skipping any that
// fail to parse (ValidateForwarded already rejected those at boot).
func parsePrefixes(cidrs []string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		if p, err := netip.ParsePrefix(c); err == nil {
			out = append(out, p.Masked())
		}
	}
	return out
}

// peerInTrustedCIDRs reports whether the TCP peer (remoteAddr, "ip:port") falls
// inside any trusted prefix. Malformed addresses return false (fail-closed);
// IPv4-mapped IPv6 peers are unmapped so an IPv4 prefix matches.
func peerInTrustedCIDRs(remoteAddr string, prefixes []netip.Prefix) bool {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// normalizeProto extracts the first, lowercased token of an X-Forwarded-Proto
// header and returns it only if it is a recognised scheme ("https"/"http").
func normalizeProto(raw string) string {
	if i := strings.IndexByte(raw, ','); i >= 0 {
		raw = raw[:i]
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "https":
		return "https"
	case "http":
		return "http"
	default:
		return ""
	}
}

// forwardedHeadersMiddleware trusts a reverse proxy's X-Forwarded-Proto (and,
// when enabled, X-Forwarded-Host) ONLY when the direct TCP peer is inside the
// configured trusted-CIDR set. It is upgrade-only: it never clears an existing
// r.TLS, so a genuine TLS connection cannot be downgraded by a forwarded http
// header. When no peers are trusted (mode=off) it is a no-op passthrough.
func forwardedHeadersMiddleware(cfg app.ForwardedConfig) func(http.Handler) http.Handler {
	prefixes := resolveTrustedPrefixes(cfg)
	if len(prefixes) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	trustHost := cfg.TrustHost
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if peerInTrustedCIDRs(r.RemoteAddr, prefixes) {
				if proto := normalizeProto(r.Header.Get("X-Forwarded-Proto")); proto != "" {
					// The scheme is signaled via the context flag (read by
					// burrow.RequestIsHTTPS) and the r.TLS sentinel (read by
					// unrolled/secure's isSSL). We deliberately do NOT touch
					// r.URL.Scheme: a server request's URL is origin-form (no
					// host), so setting only the scheme would make
					// r.URL.String() return "https:///path" and corrupt logs
					// and any absolute URL built from r.URL.
					r = r.WithContext(app.WithForwardedProto(r.Context(), proto))
					if proto == "https" && r.TLS == nil {
						// Sentinel: consumers must only check r.TLS != nil,
						// never read certificate fields off it.
						r.TLS = &tls.ConnectionState{}
					}
				}
				if trustHost {
					if host := r.Header.Get("X-Forwarded-Host"); host != "" {
						r.Host = host
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
