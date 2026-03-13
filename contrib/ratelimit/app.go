package ratelimit

import (
	"context"
	"net/http"
	"time"

	"github.com/oliverandrich/burrow"
	"github.com/urfave/cli/v3"
)

// Option configures the rate limiter app.
type Option func(*App)

// WithKeyFunc sets a custom function to extract the rate limit key from a request.
// By default, the client IP is used.
func WithKeyFunc(fn func(*http.Request) string) Option {
	return func(a *App) { a.keyFunc = fn }
}

// WithOnLimited sets a custom handler for rate-limited requests.
// By default, a plain text HTTP 429 response is sent.
func WithOnLimited(fn func(http.ResponseWriter, *http.Request)) Option {
	return func(a *App) { a.onLimited = fn }
}

// New creates a new rate limiter app with the given options.
func New(opts ...Option) *App {
	a := &App{
		onLimited: defaultOnLimited,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// App implements per-client rate limiting as a burrow contrib app.
type App struct {
	limiter   *Limiter
	keyFunc   func(*http.Request) string
	onLimited func(http.ResponseWriter, *http.Request)
}

func (a *App) Name() string { return "ratelimit" }

func (a *App) Register(_ *burrow.AppConfig) error { return nil }

func (a *App) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
	return []cli.Flag{
		&cli.FloatFlag{
			Name:    "ratelimit-rate",
			Value:   10,
			Usage:   "Requests per second (token refill rate)",
			Sources: burrow.FlagSources(configSource, "RATELIMIT_RATE", "ratelimit.rate"),
		},
		&cli.IntFlag{
			Name:    "ratelimit-burst",
			Value:   20,
			Usage:   "Maximum burst size (bucket capacity)",
			Sources: burrow.FlagSources(configSource, "RATELIMIT_BURST", "ratelimit.burst"),
		},
		&cli.DurationFlag{
			Name:    "ratelimit-cleanup-interval",
			Value:   time.Minute,
			Usage:   "Interval for sweeping expired rate limit entries",
			Sources: burrow.FlagSources(configSource, "RATELIMIT_CLEANUP_INTERVAL", "ratelimit.cleanup_interval"),
		},
		&cli.BoolFlag{
			Name:    "ratelimit-trust-proxy",
			Usage:   "Use X-Real-IP header for client IP extraction",
			Sources: burrow.FlagSources(configSource, "RATELIMIT_TRUST_PROXY", "ratelimit.trust_proxy"),
		},
		&cli.IntFlag{
			Name:    "ratelimit-max-clients",
			Value:   10000,
			Usage:   "Maximum number of tracked clients (0 = unlimited)",
			Sources: burrow.FlagSources(configSource, "RATELIMIT_MAX_CLIENTS", "ratelimit.max_clients"),
		},
	}
}

func (a *App) Configure(cmd *cli.Command) error {
	rps := cmd.Float("ratelimit-rate")
	burst := int(cmd.Int("ratelimit-burst"))
	trustProxy := cmd.Bool("ratelimit-trust-proxy")
	cleanupInterval := cmd.Duration("ratelimit-cleanup-interval")
	maxClients := int(cmd.Int("ratelimit-max-clients"))

	a.configureWithCleanup(rps, burst, trustProxy, cleanupInterval, maxClients)
	return nil
}

// configure sets up the limiter with default cleanup interval.
// Used by tests that don't need a cli.Command.
func (a *App) configure(rps float64, burst int, trustProxy bool) {
	a.configureWithCleanup(rps, burst, trustProxy, time.Minute, 0)
}

func (a *App) configureWithCleanup(rps float64, burst int, trustProxy bool, cleanupInterval time.Duration, maxClients int) {
	if a.keyFunc == nil {
		a.keyFunc = defaultKeyFunc(trustProxy)
	}
	a.limiter = NewLimiter(rps, burst, cleanupInterval, maxClients)
}

func (a *App) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{a.rateLimitMiddleware}
}

// Shutdown stops the limiter's cleanup goroutine.
func (a *App) Shutdown(_ context.Context) error {
	if a.limiter != nil {
		a.limiter.Stop()
	}
	return nil
}
