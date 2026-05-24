package selfupdate

import (
	"net/http"

	"github.com/oliverandrich/burrow"
	"github.com/urfave/cli/v3"
)

// App is the contrib app. Construct via [New].
type App struct {
	owner          string
	repo           string
	host           string
	currentVersion string
	assetPattern   string
	assetMatcher   AssetMatcher
	binaryName     string
	checksumFile   string
	http           *http.Client
}

// New constructs the selfupdate app. WithRepo, WithHost, and
// WithCurrentVersion are required; the rest take defaults.
func New(opts ...Option) *App {
	a := &App{
		assetPattern: defaultAssetPattern,
		checksumFile: defaultChecksumFile,
		http:         newDefaultHTTPClient(),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

const defaultChecksumFile = "checksums.txt"

// Name implements burrow.App.
func (a *App) Name() string { return "selfupdate" }

// Flags exposes runtime overrides — only the host, so users can
// redirect to a mirror or a self-hosted Forgejo instance without
// recompiling.
func (a *App) Flags(configSource func(key string) cli.ValueSource) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "update-host",
			Usage:   "Override the configured SCM host (e.g. codeberg.org, forgejo.example.com).",
			Sources: burrow.FlagSources(configSource, "UPDATE_HOST", "selfupdate.host"),
		},
	}
}

// Configure applies the optional --update-host flag override.
// Implements [burrow.Configurable].
//
// Validation of the required WithRepo / WithHost / WithCurrentVersion
// options is intentionally deferred to the `update` sub-command's
// action — failing here would break the entire boot sequence (and
// every other CLI command, including `serve`) when selfupdate is
// registered but only partially configured.
func (a *App) Configure(_ *burrow.AppConfig, cmd *cli.Command) error {
	if v := cmd.String("update-host"); v != "" {
		a.host = v
	}
	return nil
}

// CLICommands registers the `update` sub-command. Implements
// [burrow.HasCLICommands].
func (a *App) CLICommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:  "update",
			Usage: "Download and install the latest release for this platform.",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "check",
					Usage: "Report whether an update is available; do not apply.",
				},
				&cli.StringFlag{
					Name:  "to",
					Usage: "Pin to a specific release tag (e.g. v1.2.3).",
				},
			},
			Action: a.updateAction,
		},
	}
}
