package selfupdate

import "net/http"

// AssetMatcher selects the release asset to download. It receives
// the asset's filename and returns true on a match. Wrap a compiled
// regex with `re.MatchString` for regex-based matching.
type AssetMatcher func(name string) bool

// Option configures the selfupdate app via [New].
type Option func(*App)

// WithRepo sets the owner/repo coordinates. Required.
func WithRepo(owner, repo string) Option {
	return func(a *App) {
		a.owner = owner
		a.repo = repo
	}
}

// WithHost sets the SCM host. Examples: "github.com",
// "codeberg.org", "forgejo.example.com". Required.
func WithHost(host string) Option {
	return func(a *App) { a.host = host }
}

// WithCurrentVersion supplies the running binary's version, used to
// compare against the latest release tag. Pass the project's own
// `main.version` (set by goreleaser via `-ldflags="-X main.version=..."`).
// Required.
func WithCurrentVersion(version string) Option {
	return func(a *App) { a.currentVersion = version }
}

// WithAssetPattern overrides the asset-name template. Defaults to
// the goreleaser scaffold convention. Template fields: Name, Version,
// OS, Arch, ArchAlias, Ext. Ignored when [WithAssetMatcher] is set.
func WithAssetPattern(tmpl string) Option {
	return func(a *App) { a.assetPattern = tmpl }
}

// WithAssetMatcher overrides asset selection with a predicate. The
// release's assets are walked in order; the first whose name returns
// true is downloaded. Use this when the goreleaser-style template
// doesn't fit — for example, with `regexp.MustCompile(...).MatchString`.
// Takes precedence over [WithAssetPattern].
func WithAssetMatcher(fn AssetMatcher) Option {
	return func(a *App) { a.assetMatcher = fn }
}

// WithBinaryName overrides the binary's filename inside the archive.
// Defaults to the repo name (with `.exe` on Windows). Useful when
// goreleaser is configured with `binary:` set to a value that
// differs from the repo name.
func WithBinaryName(name string) Option {
	return func(a *App) { a.binaryName = name }
}

// WithChecksumFile overrides the checksum file name. Defaults to
// "checksums.txt" (goreleaser scaffold convention).
func WithChecksumFile(name string) Option {
	return func(a *App) { a.checksumFile = name }
}

// WithHTTPClient injects a custom HTTP client — primarily for tests
// (httptest.Server.Client()) and proxied environments.
func WithHTTPClient(hc *http.Client) Option {
	return func(a *App) { a.http = hc }
}
