package csrf

import (
	"html/template"
	"net/http"
	"strings"

	gorillacsrf "github.com/gorilla/csrf"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/internal/cryptokey"
	"github.com/urfave/cli/v3"
)

// New creates a new CSRF app.
func New() *App { return &App{} }

// App implements CSRF protection as a burrow contrib app.
type App struct {
	config  *burrow.Config
	protect func(http.Handler) http.Handler
	secure  bool
}

func (a *App) Name() string { return "csrf" }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.config = cfg.Config
	return nil
}

func (a *App) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "csrf-key",
			Usage:   "CSRF auth key (32-byte hex, auto-generated if empty)",
			Sources: burrow.FlagSources(configSource, "CSRF_KEY", "csrf.key"),
		},
	}
}

func (a *App) Configure(cmd *cli.Command) error {
	keyHex := cmd.String("csrf-key")
	secure := a.config != nil && strings.HasPrefix(a.config.Server.BaseURL, "https://")
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
	)
	return nil
}

// RequestFuncMap returns request-scoped template functions for CSRF tokens.
func (a *App) RequestFuncMap(r *http.Request) template.FuncMap {
	return template.FuncMap{
		"csrfToken": func() string { return Token(r.Context()) },
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
