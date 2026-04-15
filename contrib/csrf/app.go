package csrf

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"

	gorillacsrf "github.com/gorilla/csrf"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/internal/cryptokey"
	"github.com/urfave/cli/v3"
)

// New creates a new CSRF app.
func New() *App { return &App{} }

// App implements CSRF protection as a burrow contrib app.
type App struct {
	protect func(http.Handler) http.Handler
	secure  bool
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
	return a.configure(keyHex, secure)
}

// configure sets up the gorilla/csrf middleware with the given key and secure flag.
// Extracted for testability without requiring a cli.Command.
func (a *App) configure(keyHex string, secure bool) error {
	key, err := cryptokey.Resolve(keyHex, "csrf")
	if err != nil {
		return err
	}

	a.secure = secure
	a.protect = gorillacsrf.Protect(
		key,
		gorillacsrf.Secure(secure),
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

	// gorilla/csrf assumes HTTPS by default. For plaintext HTTP deployments,
	// mark each request so gorilla skips HTTPS-only referer checks.
	if !a.secure {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapped.ServeHTTP(w, gorillacsrf.PlaintextHTTPRequest(r))
		})
	}

	return wrapped
}
