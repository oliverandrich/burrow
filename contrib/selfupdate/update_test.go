package selfupdate

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestUpdateAction_CheckReportsAvailableUpdate(t *testing.T) {
	srv := fakeReleaseServer(t, "v1.2.3", validArchive(t))
	t.Cleanup(srv.Close)

	app := newConfiguredApp(srv)
	out := runUpdate(t, app, []string{"update", "--check"})
	assert.Contains(t, out, "update available v1.0.0 → v1.2.3")
}

func TestUpdateAction_CheckSaysAlreadyOnLatest(t *testing.T) {
	srv := fakeReleaseServer(t, "v1.0.0", nil)
	t.Cleanup(srv.Close)

	app := newConfiguredApp(srv)
	out := runUpdate(t, app, []string{"update", "--check"})
	assert.Contains(t, out, "already on latest")
}

func TestUpdateAction_ToSameTagShortCircuits(t *testing.T) {
	srv := fakeReleaseServer(t, "v1.0.0", validArchive(t))
	t.Cleanup(srv.Close)

	app := newConfiguredApp(srv)
	out := runUpdate(t, app, []string{"update", "--to", "v1.0.0"})
	assert.Contains(t, out, "already on v1.0.0", "pin-to-same-tag must not re-download")
}

func TestUpdateAction_RefusesCorruptedChecksum(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install path uses POSIX file semantics; not testing on windows")
	}
	archive := validArchive(t)
	wrong := sha256.Sum256([]byte("not the archive"))
	srv := fakeReleaseServerCustom(t, "v2.0.0", archive, fmt.Sprintf(
		"%s  %s\n",
		hex.EncodeToString(wrong[:]),
		conventionalAssetName("myapp", "2.0.0"),
	))
	t.Cleanup(srv.Close)

	app := newConfiguredApp(srv)
	cmd := buildUpdateCommand(t, app)
	err := cmd.Run(t.Context(), []string{"app", "update"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
}

func TestUpdateAction_NoMatchingAsset(t *testing.T) {
	// Release JSON contains a checksum file and an asset, but the
	// asset name doesn't match the pattern we'd generate.
	srv := fakeReleaseServerWithAssetName(t, "v1.5.0", "wrong-name.tar.gz", validArchive(t))
	t.Cleanup(srv.Close)

	app := newConfiguredApp(srv)
	cmd := buildUpdateCommand(t, app)
	err := cmd.Run(t.Context(), []string{"app", "update", "--check"})
	// --check reaches the asset-name resolution but doesn't error
	// when the asset is missing from the release (it just reports
	// the name it'd look for).
	require.NoError(t, err)
}

func TestUpdateAction_AssetMatcherOverridesPattern(t *testing.T) {
	// Release ships an asset under a non-conventional name. The
	// matcher option lets us select it without rewriting the
	// pattern template.
	customName := "wholly-different-name.tar.gz"
	srv := fakeReleaseServerWithAssetName(t, "v1.5.0", customName, validArchive(t))
	t.Cleanup(srv.Close)

	app := New(
		WithRepo("me", "myapp"),
		WithHost(srv.URL),
		WithCurrentVersion("v1.0.0"),
		WithHTTPClient(srv.Client()),
		WithAssetMatcher(func(name string) bool {
			return strings.HasPrefix(name, "wholly-different")
		}),
	)
	require.NoError(t, app.Configure(&burrow.AppConfig{}, &cli.Command{}))

	out := runUpdate(t, app, []string{"update", "--check"})
	assert.Contains(t, out, customName)
}

func TestUpdateAction_MissingRepoErrorsAtActionNotConfigure(t *testing.T) {
	// Configure must succeed even when options are incomplete —
	// so that registering selfupdate doesn't break unrelated commands.
	app := New() // no options at all
	require.NoError(t, app.Configure(&burrow.AppConfig{}, &cli.Command{}))

	cmd := buildUpdateCommand(t, app)
	err := cmd.Run(t.Context(), []string{"app", "update"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WithRepo is required")
}

// --- helpers ---

// newConfiguredApp builds a selfupdate App pointing at srv with a
// fixed current version of v1.0.0. Tests that need a different
// version construct the App directly via New(...).
func newConfiguredApp(srv *httptest.Server) *App {
	return New(
		WithRepo("me", "myapp"),
		WithHost(srv.URL),
		WithCurrentVersion("v1.0.0"),
		WithHTTPClient(srv.Client()),
	)
}

func validArchive(t *testing.T) []byte {
	t.Helper()
	return makeTarGz(t, []tarEntry{
		{name: binaryName("myapp", runtime.GOOS), body: []byte("real binary contents"), typ: tar.TypeReg},
	})
}

// fakeReleaseServer mounts a Gitea-shaped /releases/latest endpoint
// plus asset download URLs that point back at itself, returning the
// supplied archive bytes for the conventional asset name. Checksum
// file is computed from the archive.
func fakeReleaseServer(t *testing.T, tag string, archive []byte) *httptest.Server {
	t.Helper()
	if archive == nil {
		return fakeReleaseServerCustom(t, tag, nil, "")
	}
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%s  %s\n",
		hex.EncodeToString(sum[:]),
		conventionalAssetName("myapp", strings.TrimPrefix(tag, "v")))
	return fakeReleaseServerCustom(t, tag, archive, checksums)
}

// fakeReleaseServerWithAssetName lets a test override the asset name
// declared in the release JSON without changing the body.
func fakeReleaseServerWithAssetName(t *testing.T, tag, assetName string, archive []byte) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), assetName)
	return fakeReleaseServerCustomNamed(t, tag, assetName, archive, checksums)
}

func fakeReleaseServerCustom(t *testing.T, tag string, archive []byte, checksumsBody string) *httptest.Server {
	t.Helper()
	return fakeReleaseServerCustomNamed(t, tag, conventionalAssetName("myapp", strings.TrimPrefix(tag, "v")), archive, checksumsBody)
}

func fakeReleaseServerCustomNamed(t *testing.T, tag, assetName string, archive []byte, checksumsBody string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/repos/me/myapp/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{
			"tag_name": %q,
			"assets": [
				{"name": %q, "browser_download_url": "%s/dl/asset"},
				{"name": "checksums.txt", "browser_download_url": "%s/dl/checksums"}
			]
		}`, tag, assetName, srv.URL, srv.URL)
	})
	mux.HandleFunc("/api/v1/repos/me/myapp/releases/tags/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{
			"tag_name": %q,
			"assets": [
				{"name": %q, "browser_download_url": "%s/dl/asset"},
				{"name": "checksums.txt", "browser_download_url": "%s/dl/checksums"}
			]
		}`, tag, assetName, srv.URL, srv.URL)
	})
	mux.HandleFunc("/dl/asset", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/dl/checksums", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(checksumsBody))
	})
	srv = httptest.NewServer(mux)
	return srv
}

func conventionalAssetName(name, version string) string {
	return fmt.Sprintf("%s-%s-%s-%s.%s",
		name, version, runtime.GOOS, archAlias(runtime.GOARCH), archiveExt(runtime.GOOS))
}

// runUpdate invokes the update sub-command with the given args and
// returns the captured stdout. Returns "" on Action error; tests
// that expect errors should assert via buildUpdateCommand directly.
func runUpdate(t *testing.T, app *App, args []string) string {
	t.Helper()
	cmd := buildUpdateCommand(t, app)
	require.NoError(t, cmd.Run(t.Context(), append([]string{"app"}, args...)))
	return cmd.Writer.(*bytes.Buffer).String()
}

func buildUpdateCommand(t *testing.T, app *App) *cli.Command {
	t.Helper()
	out := &bytes.Buffer{}
	return &cli.Command{
		Name:     "app",
		Writer:   out,
		Commands: app.CLICommands(),
	}
}
