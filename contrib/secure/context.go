// Package secure provides security response headers as a burrow contrib app.
// It wraps [github.com/unrolled/secure] and sets sensible defaults for
// X-Content-Type-Options, X-Frame-Options, Referrer-Policy, and HSTS.
//
// By default, HSTS is enabled when the server's BaseURL uses HTTPS and
// disabled for plain HTTP. Content-Security-Policy, Permissions-Policy,
// and Cross-Origin-Opener-Policy are not set unless explicitly configured,
// as no safe universal default exists for these headers.
package secure
