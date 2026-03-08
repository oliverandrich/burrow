package session

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// --- Shared test helpers ---

var testHashKey = hex.EncodeToString(make([]byte, 32))

func configuredApp(t *testing.T) *App {
	t.Helper()
	app := New()
	_ = app.Register(&burrow.AppConfig{})

	cmd := &cli.Command{
		Name:  "test",
		Flags: app.Flags(nil),
		Action: func(_ context.Context, cmd *cli.Command) error {
			return app.Configure(cmd)
		},
	}
	err := cmd.Run(t.Context(), []string{
		"test",
		"--session-hash-key", testHashKey,
	})
	require.NoError(t, err)
	return app
}

// routerWithSession creates a chi router with session middleware.
func routerWithSession(t *testing.T) (chi.Router, *App) {
	t.Helper()
	app := configuredApp(t)
	r := chi.NewRouter()
	for _, mw := range app.Middleware() {
		r.Use(mw)
	}
	return r, app
}

// --- Compile-time interface assertions ---

var (
	_ burrow.App           = (*App)(nil)
	_ burrow.Configurable  = (*App)(nil)
	_ burrow.HasMiddleware = (*App)(nil)
)

// --- App tests ---

func TestAppName(t *testing.T) {
	app := New()
	assert.Equal(t, "session", app.Name())
}

func TestAppFlags(t *testing.T) {
	app := New()
	flags := app.Flags(nil)

	names := make(map[string]bool)
	for _, f := range flags {
		names[f.Names()[0]] = true
	}

	assert.True(t, names["session-cookie-name"])
	assert.True(t, names["session-max-age"])
	assert.True(t, names["session-hash-key"])
	assert.True(t, names["session-block-key"])
}

func TestConfigureCreatesManager(t *testing.T) {
	app := configuredApp(t)
	require.NotNil(t, app.Manager())
}
