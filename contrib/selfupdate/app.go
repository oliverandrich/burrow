// Package selfupdate adds an `update` sub-command to a burrow app
// that downloads the latest release for the running OS/arch from
// GitHub, Codeberg, or any Forgejo instance and atomically replaces
// the binary in place.
//
// Release artefacts must follow the goreleaser convention shipped in
// burrow's scaffold (`.goreleaser.yaml`): a `tar.gz` (Linux) or
// `zip` (macOS / Windows) archive named
// `<project>-<version>-<os>-<arch>.<ext>`, plus a `checksums.txt`
// SHA256 sums file. The asset-name template is overridable for
// projects that ship a different layout.
//
// No tokens, no authentication — the contrib only talks to the
// Releases API as an anonymous client. Public repos only.
//
// Wiring (in cmd/<app>/main.go):
//
//	import (
//	    "github.com/oliverandrich/burrow"
//	    "github.com/oliverandrich/burrow/contrib/selfupdate"
//	)
//
//	var version = "dev" // set by goreleaser via -ldflags="-X main.version=..."
//
//	func main() {
//	    srv := burrow.NewServer(
//	        // ... other apps ...
//	        selfupdate.New(
//	            selfupdate.WithRepo("me", "myapp"),
//	            selfupdate.WithHost("github.com"),
//	            selfupdate.WithCurrentVersion(version),
//	        ),
//	    )
//	    srv.Run()
//	}
//
// CLI surface added by registration:
//
//	myapp update            # apply latest update
//	myapp update --check    # report whether an update is available
//	myapp update --to v1.2  # pin to a specific tag (rollback)
//
// Linux is the only tested target. macOS works as a side effect of
// POSIX file semantics. Windows is best-effort and relies on
// minio/selfupdate's locked-running-binary handling.
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
