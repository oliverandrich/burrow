package registry_test

import (
	"testing"

	"github.com/oliverandrich/burrow/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeApp struct {
	name string
}

func (f *fakeApp) Name() string { return f.name }

type otherApp struct {
	name string
}

func (o *otherApp) Name() string { return o.name }

type dependentApp struct {
	name string
	deps []string
}

func (d *dependentApp) Name() string           { return d.name }
func (d *dependentApp) Dependencies() []string { return d.deps }

func TestNew_EmptyRegistry(t *testing.T) {
	reg := registry.New()
	require.NotNil(t, reg)
	assert.Empty(t, registry.Apps(reg))
}

func TestAdd_StoresApp(t *testing.T) {
	reg := registry.New()
	app := &fakeApp{name: "alpha"}

	registry.Add(reg, app)

	got, ok := registry.GetByName(reg, "alpha")
	require.True(t, ok)
	assert.Same(t, app, got)
}

func TestAdd_PreservesInsertionOrder(t *testing.T) {
	reg := registry.New()
	a := &fakeApp{name: "a"}
	b := &fakeApp{name: "b"}
	c := &fakeApp{name: "c"}

	registry.Add(reg, a)
	registry.Add(reg, b)
	registry.Add(reg, c)

	apps := registry.Apps(reg)
	require.Len(t, apps, 3)
	assert.Equal(t, "a", apps[0].Name())
	assert.Equal(t, "b", apps[1].Name())
	assert.Equal(t, "c", apps[2].Name())
}

func TestAdd_DuplicateNamePanics(t *testing.T) {
	reg := registry.New()
	registry.Add(reg, &fakeApp{name: "alpha"})

	assert.PanicsWithValue(t,
		`registry: duplicate app name "alpha"`,
		func() { registry.Add(reg, &fakeApp{name: "alpha"}) },
	)
}

func TestAdd_MissingDependencyPanics(t *testing.T) {
	reg := registry.New()

	assert.PanicsWithValue(t,
		`registry: app "dependent" requires "session" to be registered first`,
		func() {
			registry.Add(reg, &dependentApp{name: "dependent", deps: []string{"session"}})
		},
	)
}

func TestAdd_SatisfiedDependencyDoesNotPanic(t *testing.T) {
	reg := registry.New()
	registry.Add(reg, &fakeApp{name: "session"})

	assert.NotPanics(t, func() {
		registry.Add(reg, &dependentApp{name: "dependent", deps: []string{"session"}})
	})
}

func TestApps_ReturnsCopy(t *testing.T) {
	reg := registry.New()
	registry.Add(reg, &fakeApp{name: "alpha"})

	apps := registry.Apps(reg)
	apps[0] = &fakeApp{name: "tampered"}

	got, ok := registry.GetByName(reg, "alpha")
	require.True(t, ok)
	assert.Equal(t, "alpha", got.Name())
}

func TestGetByName_Hit(t *testing.T) {
	reg := registry.New()
	app := &fakeApp{name: "alpha"}
	registry.Add(reg, app)

	got, ok := registry.GetByName(reg, "alpha")
	require.True(t, ok)
	assert.Same(t, app, got)
}

func TestGetByName_Miss(t *testing.T) {
	reg := registry.New()

	got, ok := registry.GetByName(reg, "nonexistent")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestMustGetByName_Hit(t *testing.T) {
	reg := registry.New()
	app := &fakeApp{name: "alpha"}
	registry.Add(reg, app)

	got := registry.MustGetByName(reg, "alpha")
	assert.Same(t, app, got)
}

func TestMustGetByName_PanicsOnMiss(t *testing.T) {
	reg := registry.New()

	assert.PanicsWithValue(t,
		`registry: no app named "nonexistent" registered`,
		func() { registry.MustGetByName(reg, "nonexistent") },
	)
}

func TestGet_TypeKeyed_NoMatch(t *testing.T) {
	reg := registry.New()
	registry.Add(reg, &fakeApp{name: "alpha"})

	got, ok := registry.Get[*otherApp](reg)
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestGet_TypeKeyed_OneMatch(t *testing.T) {
	reg := registry.New()
	app := &fakeApp{name: "alpha"}
	registry.Add(reg, app)

	got, ok := registry.Get[*fakeApp](reg)
	require.True(t, ok)
	assert.Same(t, app, got)
}

func TestGet_TypeKeyed_MultipleMatchesReturnsFalse(t *testing.T) {
	reg := registry.New()
	registry.Add(reg, &fakeApp{name: "alpha"})
	registry.Add(reg, &fakeApp{name: "beta"})

	got, ok := registry.Get[*fakeApp](reg)
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestMustGet_TypeKeyed_OneMatch(t *testing.T) {
	reg := registry.New()
	app := &fakeApp{name: "alpha"}
	registry.Add(reg, app)

	got := registry.MustGet[*fakeApp](reg)
	assert.Same(t, app, got)
}

func TestMustGet_TypeKeyed_NoMatchPanics(t *testing.T) {
	reg := registry.New()
	registry.Add(reg, &fakeApp{name: "alpha"})

	assert.PanicsWithValue(t,
		`registry: no app of type *registry_test.otherApp registered`,
		func() { registry.MustGet[*otherApp](reg) },
	)
}

func TestMustGet_TypeKeyed_MultipleMatchesPanics(t *testing.T) {
	reg := registry.New()
	registry.Add(reg, &fakeApp{name: "alpha"})
	registry.Add(reg, &fakeApp{name: "beta"})

	assert.PanicsWithValue(t,
		`registry: multiple apps of type *registry_test.fakeApp registered`,
		func() { registry.MustGet[*fakeApp](reg) },
	)
}
