package burrow

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// stubApp is a minimal App with no shutdown support.
type stubApp struct {
	name string
}

func (a *stubApp) Name() string { return a.name }

// shutdownApp is an App that implements HasShutdown and records call order.
type shutdownApp struct { //nolint:govet // fieldalignment: readability over optimization
	name  string
	order *[]string
	err   error
}

func (a *shutdownApp) Name() string { return a.name }
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

// configurableApp tracks Configure call order.
type configurableApp struct { //nolint:govet // fieldalignment: readability over optimization
	name  string
	order *[]string
}

func (a *configurableApp) Name() string { return a.name }
func (a *configurableApp) Configure(_ *AppConfig, _ *cli.Command) error {
	*a.order = append(*a.order, a.name+".Configure")
	return nil
}

// postConfigurableApp tracks both Configure and PostConfigure call order.
type postConfigurableApp struct { //nolint:govet // fieldalignment: readability over optimization
	name  string
	order *[]string
}

func (a *postConfigurableApp) Name() string { return a.name }
func (a *postConfigurableApp) Configure(_ *AppConfig, _ *cli.Command) error {
	*a.order = append(*a.order, a.name+".Configure")
	return nil
}
func (a *postConfigurableApp) PostConfigure(_ *AppConfig, _ *cli.Command) error {
	*a.order = append(*a.order, a.name+".PostConfigure")
	return nil
}

func TestRegistryConfigure_PostConfigureRunsAfterAllConfigure(t *testing.T) {
	var order []string
	a1 := &configurableApp{name: "alpha", order: &order}
	a2 := &postConfigurableApp{name: "beta", order: &order}
	a3 := &configurableApp{name: "gamma", order: &order}

	reg := NewRegistry()
	reg.Add(a1)
	reg.Add(a2)
	reg.Add(a3)

	err := reg.Configure(&AppConfig{Registry: reg}, nil)
	require.NoError(t, err)

	// All Configure calls must happen before any PostConfigure call.
	assert.Equal(t, []string{
		"alpha.Configure",
		"beta.Configure",
		"gamma.Configure",
		"beta.PostConfigure",
	}, order)
}

func TestRegistryConfigure_PostConfigureError(t *testing.T) {
	var order []string
	a1 := &configurableApp{name: "alpha", order: &order}

	reg := NewRegistry()
	reg.Add(a1)
	reg.Add(&postConfigErrorApp{name: "failing"})

	err := reg.Configure(&AppConfig{Registry: reg}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "post-configure app \"failing\"")
}

// postConfigErrorApp returns an error from PostConfigure.
type postConfigErrorApp struct {
	name string
}

func (a *postConfigErrorApp) Name() string                                 { return a.name }
func (a *postConfigErrorApp) Configure(_ *AppConfig, _ *cli.Command) error { return nil }
func (a *postConfigErrorApp) PostConfigure(_ *AppConfig, _ *cli.Command) error {
	return errors.New("boom")
}

func TestRegistryConfigure_SkipsNonPostConfigurable(t *testing.T) {
	var order []string
	a1 := &configurableApp{name: "alpha", order: &order}
	a2 := &configurableApp{name: "beta", order: &order}

	reg := NewRegistry()
	reg.Add(a1)
	reg.Add(a2)

	err := reg.Configure(&AppConfig{Registry: reg}, nil)
	require.NoError(t, err)

	// Only Configure calls, no PostConfigure.
	assert.Equal(t, []string{"alpha.Configure", "beta.Configure"}, order)
}
