package csrf

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	gorillacsrf "github.com/gorilla/csrf"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/internal/cryptokey"
	"github.com/oliverandrich/burrow/registry"
	"github.com/urfave/cli/v3"
)

// ExemptPaths is implemented by apps that want specific URL paths to
// bypass CSRF token validation — typically webhook receivers (webmention
// inbound, ActivityPub inbox, payment callbacks) that accept POSTs from
// external services without a CSRF token by design.
//
// The csrf app discovers every implementor in the registry during boot
// and merges their declarations into a single matcher. This keeps the
// declaration local to the app that owns the route: a Webmention contrib
// returns "/webmention" from its own CSRFExemptPaths, the application's
// main.go stays unaware of which routes need bypassing.
//
// Pattern syntax (minimal by design):
//
//   - "/webmention" — exact match (matches /webmention, NOT /webmention/x).
//   - "/inbox/"     — prefix match (trailing slash; matches /inbox/alice,
//     /inbox/bob/feed, NOT /inbox).
type ExemptPaths interface {
	CSRFExemptPaths() []string
}

// New creates a new CSRF app.
func New() *App { return &App{} }

// App implements CSRF protection as a burrow contrib app.
type App struct {
	protect  func(http.Handler) http.Handler
	secure   bool
	registry *registry.Registry
	exempt   *exemptMatcher
}

func (a *App) Name() string { return "csrf" }

func (a *App) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "csrf-key",
			Usage:   "CSRF auth key (32-byte hex, auto-generated if empty)",
			Sources: burrow.FlagSources(configSource, "CSRF_KEY", "csrf.key"),
		},
	}
}

func (a *App) Configure(cfg *burrow.AppConfig, cmd *cli.Command) error {
	keyHex := cmd.String("csrf-key")
	secure := cfg.Config != nil && cfg.Config.IsHTTPS()
	a.registry = cfg.Registry
	return a.configure(keyHex, secure)
}

// configure sets up the gorilla/csrf middleware with the given key and secure flag.
// Extracted for testability without requiring a cli.Command.
func (a *App) configure(keyHex string, secure bool) error {
	key, err := cryptokey.Resolve(keyHex, "csrf")
	if err != nil {
		return err
	}

	// a.secure is the boot-time cookie Secure attribute (gorilla sets it once
	// here). The per-request HTTPS decision for the origin check lives in
	// csrfMiddleware via burrow.RequestIsHTTPS.
	a.secure = secure
	a.exempt = buildExemptMatcher(a.registry)
	a.protect = gorillacsrf.Protect(
		key,
		gorillacsrf.Secure(a.secure),
		gorillacsrf.SameSite(gorillacsrf.SameSiteLaxMode),
		gorillacsrf.Path("/"),
		gorillacsrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slog.Warn("CSRF validation failed", "reason", gorillacsrf.FailureReason(r), "method", r.Method, "path", r.URL.Path) //nolint:gosec // G706: slog structured logging is safe
			http.Error(w, "Forbidden", http.StatusForbidden)
		})),
	)
	return nil
}

// RequestFuncMap returns context-scoped template functions for CSRF tokens.
func (a *App) RequestFuncMap(ctx context.Context) template.FuncMap {
	token := Token(ctx)
	return template.FuncMap{
		"csrfToken": func() string { return token },
		"csrfField": func() template.HTML {
			return template.HTML(`<input type="hidden" name="gorilla.csrf.Token" value="` + token + `">`) //nolint:gosec // G203: token is a base64-encoded opaque value from gorilla/csrf
		},
		"csrfHxHeaders": func() template.HTMLAttr {
			return template.HTMLAttr(` hx-headers='{"X-CSRF-Token":"` + token + `"}'`) //nolint:gosec // G203: token is a base64-encoded opaque value from gorilla/csrf
		},
	}
}

func (a *App) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{a.csrfMiddleware}
}

func (a *App) csrfMiddleware(next http.Handler) http.Handler {
	// Wrap the inner handler to bridge the token into the context.
	bridged := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := gorillacsrf.Token(r)
		ctx := WithToken(r.Context(), token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})

	wrapped := a.protect(bridged)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Webhook receivers and similar cross-origin endpoints can opt
		// their path out of CSRF validation via the ExemptPaths capability
		// interface. UnsafeSkipCheck must be applied to the request
		// BEFORE gorilla.Protect runs.
		if a.exempt.matches(r.URL.Path) {
			r = gorillacsrf.UnsafeSkipCheck(r)
		}
		// gorilla/csrf assumes HTTPS by default and decides plaintext-vs-secure
		// solely from this flag. Mark genuinely-plaintext requests so it skips
		// the HTTPS-only origin/referer checks. The decision is per-request
		// (RequestIsHTTPS honors a trusted proxy's X-Forwarded-Proto), not the
		// boot-static scheme — otherwise an https request proxied to a plain-HTTP
		// app is treated as http and its https Origin is rejected.
		if !burrow.RequestIsHTTPS(r) {
			r = gorillacsrf.PlaintextHTTPRequest(r)
		}
		wrapped.ServeHTTP(w, r)
	})
}

// exemptMatcher is the merged exempt-path lookup table built once at
// boot from every registry app that implements ExemptPaths. The matcher
// is safe to call when nil — it returns false for every path.
type exemptMatcher struct {
	exact  map[string]struct{}
	prefix []string // each entry already validated to end with "/"
}

// buildExemptMatcher walks the registry collecting every ExemptPaths
// declaration. Returns nil when the registry is nil (e.g. tests that
// construct the app via configure() without a Configure call) or when
// no app implements the interface — both are normal modes.
func buildExemptMatcher(reg *registry.Registry) *exemptMatcher {
	if reg == nil {
		return nil
	}
	m := &exemptMatcher{exact: map[string]struct{}{}}
	for _, app := range registry.Apps(reg) {
		provider, ok := app.(ExemptPaths)
		if !ok {
			continue
		}
		for _, p := range provider.CSRFExemptPaths() {
			if p == "" {
				continue
			}
			if strings.HasSuffix(p, "/") {
				m.prefix = append(m.prefix, p)
			} else {
				m.exact[p] = struct{}{}
			}
		}
	}
	if len(m.exact) == 0 && len(m.prefix) == 0 {
		return nil
	}
	return m
}

// matches reports whether path is exempt from CSRF validation. The
// receiver may be nil — that case is treated as "no exemptions".
func (m *exemptMatcher) matches(path string) bool {
	if m == nil {
		return false
	}
	if _, ok := m.exact[path]; ok {
		return true
	}
	for _, p := range m.prefix {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
