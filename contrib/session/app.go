package session

import (
	"context"
	"net/http"
	"strings"

	"codeberg.org/oliverandrich/burrow"
	"github.com/gorilla/securecookie"
	"github.com/urfave/cli/v3"
)

// New creates a new session app.
func New() *App { return &App{} }

// App implements the session contrib app.
type App struct {
	manager *Manager
	config  *burrow.Config
}

func (a *App) Name() string { return "session" }

func (a *App) Register(cfg *burrow.AppConfig) error {
	a.config = cfg.Config
	return nil
}

func (a *App) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "session-cookie-name",
			Value:   "_session",
			Usage:   "Session cookie name",
			Sources: cli.EnvVars("SESSION_COOKIE_NAME"),
		},
		&cli.IntFlag{
			Name:    "session-max-age",
			Value:   604800, // 7 days
			Usage:   "Session max age in seconds",
			Sources: cli.EnvVars("SESSION_MAX_AGE"),
		},
		&cli.StringFlag{
			Name:    "session-hash-key",
			Usage:   "Session hash key (32-byte hex, auto-generated if empty)",
			Sources: cli.EnvVars("SESSION_HASH_KEY"),
		},
		&cli.StringFlag{
			Name:    "session-block-key",
			Usage:   "Session block key for encryption (32-byte hex, optional)",
			Sources: cli.EnvVars("SESSION_BLOCK_KEY"),
		},
	}
}

func (a *App) Configure(cmd *cli.Command) error {
	hashKey, err := resolveKey(cmd.String("session-hash-key"), "hash")
	if err != nil {
		return err
	}

	var blockKey []byte
	if bk := cmd.String("session-block-key"); bk != "" {
		blockKey, err = decodeKey(bk, "block")
		if err != nil {
			return err
		}
	}

	cookieName := cmd.String("session-cookie-name")
	maxAge := int(cmd.Int("session-max-age"))

	secure := a.config != nil && strings.HasPrefix(a.config.Server.BaseURL, "https://")

	sc := securecookie.New(hashKey, blockKey)
	sc.MaxAge(maxAge)

	a.manager = &Manager{
		sc:         sc,
		cookieName: cookieName,
		maxAge:     maxAge,
		secure:     secure,
	}
	return nil
}

func (a *App) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{a.sessionMiddleware}
}

func (a *App) sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values, _ := a.manager.Parse(r)
		s := &state{manager: a.manager, values: values}
		ctx := context.WithValue(r.Context(), ctxKeySession{}, s)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Manager returns the session manager for other apps to use.
func (a *App) Manager() *Manager {
	return a.manager
}
