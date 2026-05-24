package selfupdate

import (
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// Compile-time interface assertions live here so the package's
// burrow contract stays explicit.
var (
	_ burrow.App            = (*App)(nil)
	_ burrow.Configurable   = (*App)(nil)
	_ burrow.HasFlags       = (*App)(nil)
	_ burrow.HasCLICommands = (*App)(nil)
)

func TestConfigure_DoesNotErrorOnMissingOptions(t *testing.T) {
	// Configure must not fail boot just because selfupdate is not
	// fully wired — that would break every other CLI command and
	// `serve`. Required-field validation belongs in the update action.
	app := New()
	require.NoError(t, app.Configure(&burrow.AppConfig{}, &cli.Command{}))
}

func TestValidate_RejectsMissingFields(t *testing.T) {
	tests := []struct {
		name    string
		app     *App
		wantSub string
	}{
		{"no repo", New(WithHost("github.com"), WithCurrentVersion("v1.0.0")), "WithRepo is required"},
		{"no host", New(WithRepo("me", "x"), WithCurrentVersion("v1.0.0")), "WithHost is required"},
		{"no version", New(WithRepo("me", "x"), WithHost("github.com")), "WithCurrentVersion is required"},
		{"invalid version", New(WithRepo("me", "x"), WithHost("github.com"), WithCurrentVersion("not-semver")), "not valid semver"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.app.validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantSub)
		})
	}
}

func TestValidate_AcceptsValidConfig(t *testing.T) {
	app := New(
		WithRepo("me", "x"),
		WithHost("github.com"),
		WithCurrentVersion("v1.0.0"),
	)
	require.NoError(t, app.validate())
}

func TestEffectiveBinaryName(t *testing.T) {
	a := New(WithRepo("me", "go-myapp-server"))
	defaultName := a.effectiveBinaryName()
	assert.Contains(t, []string{"go-myapp-server", "go-myapp-server.exe"}, defaultName)

	b := New(WithRepo("me", "go-myapp-server"), WithBinaryName("myapp"))
	custom := b.effectiveBinaryName()
	assert.Contains(t, []string{"myapp", "myapp.exe"}, custom, "WithBinaryName overrides the repo-derived default")
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name, current, latest, to string
		want                      versionVerdict
	}{
		{"older than latest", "v1.0.0", "v1.2.0", "", verdictProceed},
		{"equal to latest", "v1.2.0", "v1.2.0", "", verdictAlreadyOnLatest},
		{"newer than latest", "v1.5.0", "v1.2.0", "", verdictAlreadyOnLatest},
		{"pin to same tag", "v1.2.0", "v1.2.0", "v1.2.0", verdictAlreadyOnRequested},
		{"pin to older tag (rollback)", "v1.5.0", "v1.0.0", "v1.0.0", verdictProceed},
		{"pin to newer tag", "v1.0.0", "v1.5.0", "v1.5.0", verdictProceed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, compareVersions(tt.current, tt.latest, tt.to))
		})
	}
}
