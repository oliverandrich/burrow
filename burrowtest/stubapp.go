package burrowtest

import "github.com/oliverandrich/burrow"

// StubApp returns a minimal [burrow.App] that exposes only the given name.
// Use it in tests to satisfy a contrib's [burrow.HasDependencies] declaration
// when the test doesn't actually exercise the depended-on contrib's behaviour.
func StubApp(name string) burrow.App {
	return &stubApp{name: name}
}

type stubApp struct{ name string }

func (s *stubApp) Name() string { return s.name }
