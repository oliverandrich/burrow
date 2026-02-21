// Package csrf provides CSRF protection as a burrow contrib app.
// It wraps gorilla/csrf and provides context helpers for reading
// the CSRF token in templates.
package csrf

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"codeberg.org/oliverandrich/burrow"
	gorillacsrf "github.com/gorilla/csrf"
	"github.com/urfave/cli/v3"
)

// ctxKeyCSRFToken is the context key for the CSRF token.
type ctxKeyCSRFToken struct{}

// WithToken stores a CSRF token in the context.
func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, ctxKeyCSRFToken{}, token)
}

// Token retrieves the CSRF token from the context.
func Token(ctx context.Context) string {
	if token, ok := ctx.Value(ctxKeyCSRFToken{}).(string); ok {
		return token
	}
	return ""
}

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

func (a *App) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "csrf-key",
			Usage:   "CSRF auth key (32-byte hex, auto-generated if empty)",
			Sources: cli.EnvVars("CSRF_KEY"),
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
	key, err := resolveKey(keyHex)
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

// --- Key utilities ---

func resolveKey(keyHex string) ([]byte, error) {
	if keyHex != "" {
		return decodeKey(keyHex)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, errors.New("csrf: failed to generate auth key")
	}
	slog.Warn("No CSRF key configured, using random key (tokens will not persist across restarts)",
		"generated_key", hex.EncodeToString(key),
	)
	return key, nil
}

func decodeKey(keyHex string) ([]byte, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, errors.New("csrf: invalid key: must be hex encoded")
	}
	if len(key) != 32 {
		return nil, errors.New("csrf: invalid key: must be 32 bytes")
	}
	return key, nil
}
