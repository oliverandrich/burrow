package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ burrow.App           = (*App)(nil)
	_ burrow.Configurable  = (*App)(nil)
	_ burrow.HasMiddleware = (*App)(nil)
	_ burrow.HasShutdown   = (*App)(nil)
)

func TestApp_Name(t *testing.T) {
	app := New()
	assert.Equal(t, "sse", app.Name())
}

func TestApp_Register(t *testing.T) {
	app := New()
	err := app.Register(&burrow.AppConfig{})
	require.NoError(t, err)
}

func TestApp_BrokerNilBeforeConfigure(t *testing.T) {
	app := New()
	assert.Nil(t, app.Broker())
}

func TestApp_ShutdownBeforeConfigure(t *testing.T) {
	app := New()
	err := app.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestWithBroker_BrokerFrom(t *testing.T) {
	b := NewEventBroker(16)
	defer b.Close()

	ctx := WithBroker(context.Background(), b)
	got := Broker(ctx)
	assert.Equal(t, b, got)
}

func TestBroker_Missing(t *testing.T) {
	got := Broker(context.Background())
	assert.Nil(t, got)
}

func TestApp_MiddlewareInjectsBroker(t *testing.T) {
	app := New()
	app.broker = NewEventBroker(16)
	defer app.broker.Close()

	mw := app.Middleware()
	require.Len(t, mw, 1)

	var got *EventBroker
	handler := mw[0](http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = Broker(r.Context())
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, app.broker, got)
}
