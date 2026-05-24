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
