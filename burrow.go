package burrow

import (
	"context"
	"html/template"

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
	DB         *bun.DB
	Registry   *Registry
	Config     *Config
	WithLocale func(ctx context.Context, lang string) context.Context
}

// NavItem represents a navigation entry contributed by an app.
type NavItem struct { //nolint:govet // fieldalignment: readability over optimization
	Label     string
	LabelKey  string // i18n message ID; translated at render time, falls back to Label
	URL       string
	Icon      template.HTML
	Position  int
	AuthOnly  bool
	AdminOnly bool
}

// NavLink is a template-ready navigation item with pre-computed active state.
// It is produced by the navLinks template function from the registered NavItems,
// filtered by the current user's authentication/authorization state.
type NavLink struct {
	Label    string
	URL      string
	Icon     template.HTML
	IsActive bool
}
