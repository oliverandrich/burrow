package selfupdate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/urfave/cli/v3"
	"golang.org/x/mod/semver"
)

// maxArchiveBytes caps how large a release archive may be before
// downloadAll refuses. Generous enough for a full Go binary archive
// (typical: 10–50 MB) but bounded so a hostile mirror can't OOM us.
const maxArchiveBytes = 512 << 20 // 512 MiB

// maxChecksumBytes caps how large the checksums file may be. A
// healthy goreleaser checksums.txt with a handful of assets is well
// under 1 KiB; the cap is large enough for hundreds of assets.
const maxChecksumBytes = 1 << 20 // 1 MiB

// updateAction is the body of `<app> update`. It validates the
// configured options, fetches a release descriptor (latest, or --to
// <tag>), compares it against the current version, and — unless
// --check — downloads the matching asset, verifies the checksum,
// extracts the binary, and applies it in place via [install].
func (a *App) updateAction(ctx context.Context, cmd *cli.Command) error {
	out := stdoutFromCmd(cmd)

	if err := a.validate(); err != nil {
		return err
	}

	release, unlock, err := a.takeLock()
	if err != nil {
		return err
	}
	defer release()
	_ = unlock // referenced below; alias keeps godoc clean.

	client := newReleaseClient(a.host, a.owner, a.repo, a.http)

	requestedTag := cmd.String("to")
	var rel Release
	if requestedTag != "" {
		rel, err = client.FetchByTag(ctx, requestedTag)
	} else {
		rel, err = client.FetchLatest(ctx)
	}
	if err != nil {
		return err
	}

	switch verdict := compareVersions(a.currentVersion, rel.TagName, requestedTag); verdict {
	case verdictAlreadyOnLatest:
		_, _ = fmt.Fprintf(out, "selfupdate: already on latest (%s)\n", a.currentVersion)
		return nil
	case verdictAlreadyOnRequested:
		_, _ = fmt.Fprintf(out, "selfupdate: already on %s\n", rel.TagName)
		return nil
	case verdictProceed:
		// fall through
	}

	assetName, err := a.resolveAssetName(rel)
	if err != nil {
		return err
	}

	if cmd.Bool("check") {
		_, _ = fmt.Fprintf(out, "selfupdate: update available %s → %s (asset %s)\n",
			a.currentVersion, rel.TagName, assetName)
		return nil
	}

	assetURL, ok := findAssetURL(rel, assetName)
	if !ok {
		return fmt.Errorf("selfupdate: no asset named %q in release %s", assetName, rel.TagName)
	}
	sumsURL, ok := findAssetURL(rel, a.checksumFile)
	if !ok {
		return fmt.Errorf("selfupdate: no %s in release %s", a.checksumFile, rel.TagName)
	}

	// Fetch + parse checksums first so we fail fast on a release that
	// doesn't list our asset, before spending bandwidth on a large
	// archive download.
	sumsBlob, err := downloadAll(ctx, a.http, sumsURL, maxChecksumBytes)
	if err != nil {
		return err
	}
	sums, err := parseChecksums(bytes.NewReader(sumsBlob))
	if err != nil {
		return err
	}
	sum, err := checksumFor(sums, assetName)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "selfupdate: downloading %s\n", assetName)
	archive, err := downloadAll(ctx, a.http, assetURL, maxArchiveBytes)
	if err != nil {
		return err
	}

	// Verify the archive checksum BEFORE handing the bytes to the
	// tar/zip parser. Skipping this order would mean a tampered
	// archive's gzip/tar/zip header is parsed (possibly with a
	// memory-amplification bomb) on un-authenticated bytes.
	if err := verifySha256(archive, sum); err != nil {
		return err
	}

	binary, err := extractBinary(bytes.NewReader(archive), int64(len(archive)),
		a.effectiveBinaryName(), archiveExt(runtime.GOOS))
	if err != nil {
		return err
	}
	defer func() { _ = binary.Close() }()

	if err := install(binary, "", nil); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "selfupdate: installed %s — restart this binary to run the new version\n", rel.TagName)
	return nil
}

// validate enforces the required options at action time (not at
// Configure time, so a partially-configured selfupdate registration
// doesn't break unrelated commands at boot).
func (a *App) validate() error {
	switch {
	case a.owner == "" || a.repo == "":
		return errors.New("selfupdate: WithRepo is required")
	case a.host == "":
		return errors.New("selfupdate: WithHost is required")
	case a.currentVersion == "":
		return errors.New("selfupdate: WithCurrentVersion is required")
	}
	if !semver.IsValid(canonicaliseVersion(a.currentVersion)) {
		return fmt.Errorf("selfupdate: WithCurrentVersion %q is not valid semver — pass a tag like v1.2.3 (or v1.2.3-rc1)", a.currentVersion)
	}
	return nil
}

// takeLock acquires the per-binary update lock; release() is the
// teardown the caller must defer. The second return value is the
// same release for callers that prefer two variables.
func (a *App) takeLock() (func(), func(), error) {
	release, err := acquireUpdateLock()
	if err != nil {
		return nil, nil, err
	}
	return release, release, nil
}

// resolveAssetName picks the asset to download. When [AssetMatcher]
// is set it walks the release's assets and returns the first match;
// otherwise it renders the goreleaser-style template.
func (a *App) resolveAssetName(rel Release) (string, error) {
	if a.assetMatcher != nil {
		for _, asset := range rel.Assets {
			if a.assetMatcher(asset.Name) {
				return asset.Name, nil
			}
		}
		return "", fmt.Errorf("selfupdate: no asset matched the configured WithAssetMatcher in release %s", rel.TagName)
	}
	return resolveAssetName(a.assetPattern, assetVars{
		Name:    a.repo,
		Version: rel.TagName,
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	})
}

// effectiveBinaryName returns the configured [WithBinaryName], or
// falls back to the repo name (with `.exe` on Windows).
func (a *App) effectiveBinaryName() string {
	if a.binaryName != "" {
		if runtime.GOOS == "windows" && !strings.HasSuffix(a.binaryName, ".exe") {
			return a.binaryName + ".exe"
		}
		return a.binaryName
	}
	return binaryName(a.repo, runtime.GOOS)
}

type versionVerdict int

const (
	verdictProceed versionVerdict = iota
	verdictAlreadyOnLatest
	verdictAlreadyOnRequested
)

// compareVersions decides whether an update should proceed. When
// requestedTag is non-empty (i.e. --to was set) we honour pinning:
// reinstalling the same tag short-circuits, but downgrading is
// allowed. Without --to we only proceed if the latest tag is strictly
// newer than the current version.
func compareVersions(current, latest, requestedTag string) versionVerdict {
	cur := canonicaliseVersion(current)
	lat := canonicaliseVersion(latest)
	if requestedTag != "" {
		if cur == lat {
			return verdictAlreadyOnRequested
		}
		return verdictProceed
	}
	if semver.Compare(cur, lat) >= 0 {
		return verdictAlreadyOnLatest
	}
	return verdictProceed
}

func findAssetURL(rel Release, name string) (string, bool) {
	for _, a := range rel.Assets {
		if a.Name == name {
			return a.URL, true
		}
	}
	return "", false
}

// downloadAll fetches url and returns the response body capped at
// maxBytes. Bodies that hit the cap are rejected — we'd rather error
// loudly than silently truncate a binary.
func downloadAll(ctx context.Context, hc *http.Client, url string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("selfupdate: build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("selfupdate: GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("selfupdate: GET %s: HTTP %d", url, resp.StatusCode)
	}
	// LimitReader caps allocation; read one byte past the cap so we
	// can distinguish "exactly at cap" from "exceeds cap".
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("selfupdate: read %s: %w", url, err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("selfupdate: %s exceeds size cap of %d bytes", url, maxBytes)
	}
	return body, nil
}

// canonicaliseVersion forces a leading "v" so semver.Compare —
// which requires the v-prefix — accepts both `1.2.3` and `v1.2.3`.
// Returns the input unchanged when empty.
func canonicaliseVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

// stdoutFromCmd returns the writer attached to the root command, or
// os.Stdout when the command tree was built ad-hoc (e.g. in tests).
func stdoutFromCmd(cmd *cli.Command) io.Writer {
	if cmd == nil {
		return os.Stdout
	}
	if root := cmd.Root(); root != nil && root.Writer != nil {
		return root.Writer
	}
	if cmd.Writer != nil {
		return cmd.Writer
	}
	return os.Stdout
}
