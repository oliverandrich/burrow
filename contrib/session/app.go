package session

import (
	"strings"

	"codeberg.org/oliverandrich/burrow"
	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v5"
	"github.com/urfave/cli/v3"
)

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

func (a *App) Middleware() []echo.MiddlewareFunc {
	return []echo.MiddlewareFunc{a.sessionMiddleware}
}

func (a *App) sessionMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		values, _ := a.manager.Parse(c.Request())
		s := &state{manager: a.manager, values: values}
		c.Set(storeKey, s)
		return next(c)
	}
}

// Manager returns the session manager for other apps to use.
func (a *App) Manager() *Manager {
	return a.manager
}
