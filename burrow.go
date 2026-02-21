package burrow

import "github.com/uptrace/bun"

// App is the required interface that all apps must implement.
// An app has a unique name and a Register method that receives
// the shared configuration needed to wire into the framework.
type App interface {
	Name() string
	Register(cfg *AppConfig) error
}

// AppConfig is passed to each app's Register method, providing
// access to shared framework resources.
type AppConfig struct {
	DB       *bun.DB
	Registry *Registry
	Config   *Config
}

// NavItem represents a navigation entry contributed by an app.
type NavItem struct {
	Label     string
	URL       string
	Icon      string
	Position  int
	AuthOnly  bool
	AdminOnly bool
}
