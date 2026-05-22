// Package registry holds the apps that make up a burrow Server. It is a
// pure storage container — application lifecycle (Configure, middleware,
// routes, migrations, shutdown) lives in package burrow. Most consumers
// reach the registry through [burrow.AppConfig.Registry] inside their
// app's Configure method.
//
// Typed lookup is the idiomatic way to fetch a sibling app:
//
//	a.audit = registry.MustGet[*audit.App](cfg.Registry).Repo()
//
// See docs/guide/inter-app-communication.md for the three lookup
// patterns: Soft-Discovery, Optional-Service, and Hard-Dependency.
package registry

import (
	"fmt"
)

// App is the required interface that all apps must implement. Every app
// has a unique name used for identification, migrations, and logging.
type App interface {
	Name() string
}

// HasDependencies is implemented by apps that require other apps to be
// registered first. [Add] panics if any declared dependency is missing
// at registration time.
type HasDependencies interface {
	Dependencies() []string
}

// Registry stores apps in insertion order with a name-keyed index.
type Registry struct {
	index map[string]App
	apps  []App
}

// New creates an empty Registry.
func New() *Registry {
	return &Registry{
		index: make(map[string]App),
	}
}

// Add registers an app. It panics if an app with the same name has
// already been registered or if a declared dependency is missing —
// both are programming errors caught at startup.
func Add(reg *Registry, app App) {
	name := app.Name()
	if _, exists := reg.index[name]; exists {
		panic(fmt.Sprintf("registry: duplicate app name %q", name))
	}

	if dep, ok := app.(HasDependencies); ok {
		for _, required := range dep.Dependencies() {
			if _, exists := reg.index[required]; !exists {
				panic(fmt.Sprintf("registry: app %q requires %q to be registered first", name, required))
			}
		}
	}

	reg.apps = append(reg.apps, app)
	reg.index[name] = app
}

// Apps returns all registered apps in insertion order. The returned
// slice is a copy; callers can mutate it without affecting the registry.
func Apps(reg *Registry) []App {
	result := make([]App, len(reg.apps))
	copy(result, reg.apps)
	return result
}

// findUnique scans the registry for apps that implement T. It returns
// the last match and the total match count; callers decide how to
// handle 0-match and multi-match cases.
func findUnique[T App](reg *Registry) (T, int) {
	var found T
	matches := 0
	for _, app := range reg.apps {
		if t, ok := app.(T); ok {
			found = t
			matches++
		}
	}
	return found, matches
}

// Get returns the unique app of type T. It returns the zero value and
// false when no app of type T is registered, or when more than one app
// of type T is registered.
//
// Use this for Optional-Service lookups where the dependency may or may
// not be present and the caller wants to degrade gracefully.
func Get[T App](reg *Registry) (T, bool) {
	found, matches := findUnique[T](reg)
	if matches != 1 {
		var zero T
		return zero, false
	}
	return found, true
}

// MustGet returns the unique app of type T. It panics with a message
// that names the missing type when no app of type T is registered, or
// when more than one app of type T is registered.
//
// Use this for Hard-Dependency lookups where the dependency is declared
// via [HasDependencies] and is therefore guaranteed to be present at
// boot time. A panic from MustGet indicates a programming error.
func MustGet[T App](reg *Registry) T {
	found, matches := findUnique[T](reg)
	var zero T
	switch matches {
	case 0:
		panic(fmt.Sprintf("registry: no app of type %T registered", zero))
	case 1:
		return found
	default:
		panic(fmt.Sprintf("registry: multiple apps of type %T registered", zero))
	}
}

// GetByName returns the app with the given name, or false if not
// registered. Use this for Soft-Discovery lookups where the caller
// inspects the result with a type assertion against an interface.
func GetByName(reg *Registry, name string) (App, bool) {
	app, ok := reg.index[name]
	return app, ok
}

// MustGetByName returns the app with the given name. It panics with a
// message that names the missing app when no app of that name is registered.
func MustGetByName(reg *Registry, name string) App {
	app, ok := reg.index[name]
	if !ok {
		panic(fmt.Sprintf("registry: no app named %q registered", name))
	}
	return app
}
