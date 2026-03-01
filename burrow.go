package burrow

import (
	"github.com/a-h/templ"
	"github.com/uptrace/bun"
)

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
type NavItem struct { //nolint:govet // fieldalignment: readability over optimization
	Label     string
	LabelKey  string // i18n message ID; translated at render time, falls back to Label
	URL       string
	Icon      templ.Component
	Position  int
	AuthOnly  bool
	AdminOnly bool
}
