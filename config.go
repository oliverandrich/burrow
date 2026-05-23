package burrow

import (
	"github.com/oliverandrich/burrow/app"
	"github.com/urfave/cli/v3"
)

// NewConfig creates a Config from a parsed CLI command. Wrapper around
// [app.NewConfig].
func NewConfig(cmd *cli.Command) *Config { return app.NewConfig(cmd) }

// CoreFlags returns the CLI flags for core framework configuration. Wrapper
// around [app.CoreFlags].
func CoreFlags(configSource func(key string) cli.ValueSource) []cli.Flag {
	return app.CoreFlags(configSource)
}

// FlagSources builds a cli.ValueSourceChain from an environment variable and
// an optional TOML key. Wrapper around [app.FlagSources].
func FlagSources(configSource func(key string) cli.ValueSource, envVar, tomlKey string) cli.ValueSourceChain {
	return app.FlagSources(configSource, envVar, tomlKey)
}

// IsLocalhost reports whether the host string refers to a localhost address.
// Wrapper around [app.IsLocalhost].
func IsLocalhost(host string) bool { return app.IsLocalhost(host) }
