package burrow

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubApp is a minimal App with no shutdown support.
type stubApp struct {
	name string
}

func (a *stubApp) Name() string                { return a.name }
func (a *stubApp) Register(_ *AppConfig) error { return nil }

// shutdownApp is an App that implements HasShutdown and records call order.
type shutdownApp struct { //nolint:govet // fieldalignment: readability over optimization
	name  string
	order *[]string
	err   error
}

func (a *shutdownApp) Name() string                { return a.name }
func (a *shutdownApp) Register(_ *AppConfig) error { return nil }
func (a *shutdownApp) Shutdown(_ context.Context) error {
	*a.order = append(*a.order, a.name)
	return a.err
}

func TestRegistryShutdown_ReverseOrder(t *testing.T) {
	var order []string
	a1 := &shutdownApp{name: "first", order: &order}
	a2 := &shutdownApp{name: "second", order: &order}
	a3 := &shutdownApp{name: "third", order: &order}

	reg := NewRegistry()
	reg.Add(a1)
	reg.Add(a2)
	reg.Add(a3)

	err := reg.Shutdown(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"third", "second", "first"}, order)
}

func TestRegistryShutdown_ErrorIsolation(t *testing.T) {
	var order []string
	a1 := &shutdownApp{name: "first", order: &order}
	a2 := &shutdownApp{name: "second", order: &order, err: errors.New("boom")}
	a3 := &shutdownApp{name: "third", order: &order}

	reg := NewRegistry()
	reg.Add(a1)
	reg.Add(a2)
	reg.Add(a3)

	err := reg.Shutdown(context.Background())
	require.Error(t, err)
	// All three apps should still be called despite the error.
	assert.Equal(t, []string{"third", "second", "first"}, order)
	assert.Contains(t, err.Error(), "second")
}

func TestRegistryShutdown_SkipsNonImplementing(t *testing.T) {
	var order []string
	a1 := &shutdownApp{name: "with-shutdown", order: &order}
	a2 := &stubApp{name: "no-shutdown"}

	reg := NewRegistry()
	reg.Add(a1)
	reg.Add(a2)

	err := reg.Shutdown(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"with-shutdown"}, order)
}
