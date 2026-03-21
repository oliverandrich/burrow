package sse

import (
	"context"
	"net/http"

	"github.com/oliverandrich/burrow"
	"github.com/urfave/cli/v3"
)

const defaultBufferSize = 16

// App implements the SSE contrib app with an in-memory pub/sub broker.
type App struct {
	broker *EventBroker
}

// New creates a new SSE app.
func New() *App {
	return &App{}
}

func (a *App) Name() string { return "sse" }

func (a *App) Register(_ *burrow.AppConfig) error {
	return nil
}

func (a *App) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
	return []cli.Flag{
		&cli.IntFlag{
			Name:    "sse-buffer-size",
			Value:   defaultBufferSize,
			Usage:   "Per-client event buffer capacity",
			Sources: burrow.FlagSources(configSource, "SSE_BUFFER_SIZE", "sse.buffer_size"),
		},
	}
}

func (a *App) Configure(cmd *cli.Command) error {
	bufSize := int(cmd.Int("sse-buffer-size"))
	if bufSize < 1 {
		bufSize = defaultBufferSize
	}
	a.broker = NewEventBroker(bufSize)
	return nil
}

// Broker returns the broker. Returns nil before Configure() is called.
func (a *App) Broker() *EventBroker {
	return a.broker
}

// Middleware injects the broker into every request context.
func (a *App) Middleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if a.broker != nil {
					r = r.WithContext(WithBroker(r.Context(), a.broker))
				}
				next.ServeHTTP(w, r)
			})
		},
	}
}

// Shutdown closes the broker and disconnects all clients.
func (a *App) Shutdown(_ context.Context) error {
	if a.broker != nil {
		a.broker.Close()
	}
	return nil
}
