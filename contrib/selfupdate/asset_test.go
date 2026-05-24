package selfupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchAlias(t *testing.T) {
	assert.Equal(t, "x86_64", archAlias("amd64"))
	assert.Equal(t, "arm64", archAlias("arm64"))
	assert.Equal(t, "riscv64", archAlias("riscv64"), "unknown archs passthrough")
}

func TestArchiveExt(t *testing.T) {
	assert.Equal(t, "tar.gz", archiveExt("linux"))
	assert.Equal(t, "zip", archiveExt("darwin"))
	assert.Equal(t, "zip", archiveExt("windows"))
}

func TestArchiveFormatFromName(t *testing.T) {
	tests := []struct {
		name, want string
		wantErr    bool
	}{
		{"app-1.0.0-linux-x86_64.tar.gz", "tar.gz", false},
		{"app.tgz", "tar.gz", false},
		{"goreleaser_Darwin_all.tar.gz", "tar.gz", false},
		{"app-1.0.0-windows-x86_64.zip", "zip", false},
		{"app.exe", "", true},
		{"random-name", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := archiveFormatFromName(tt.name)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBinaryName(t *testing.T) {
	assert.Equal(t, "myapp", binaryName("myapp", "linux"))
	assert.Equal(t, "myapp.exe", binaryName("myapp", "windows"))
	assert.Equal(t, "myapp", binaryName("myapp", "darwin"))
}

func TestResolveAssetName_Default(t *testing.T) {
	got, err := resolveAssetName(defaultAssetPattern, assetVars{
		Name:    "myapp",
		Version: "1.2.3",
		OS:      "linux",
		Arch:    "amd64",
	})
	require.NoError(t, err)
	assert.Equal(t, "myapp-1.2.3-linux-x86_64.tar.gz", got)
}

func TestResolveAssetName_StripsLeadingV(t *testing.T) {
	got, err := resolveAssetName(defaultAssetPattern, assetVars{
		Name:    "myapp",
		Version: "v1.2.3",
		OS:      "darwin",
		Arch:    "arm64",
	})
	require.NoError(t, err)
	assert.Equal(t, "myapp-1.2.3-darwin-arm64.zip", got, "leading v stripped to match goreleaser convention")
}

func TestResolveAssetName_CustomTemplate(t *testing.T) {
	custom := `{{ .Name }}_{{ .Version }}_{{ .OS }}_{{ .Arch }}.{{ .Ext }}`
	got, err := resolveAssetName(custom, assetVars{
		Name: "myapp", Version: "1.2.3", OS: "linux", Arch: "amd64",
	})
	require.NoError(t, err)
	assert.Equal(t, "myapp_1.2.3_linux_amd64.tar.gz", got, "custom template uses bare Arch, not ArchAlias")
}

func TestResolveAssetName_BadTemplate(t *testing.T) {
	_, err := resolveAssetName(`{{ .Bogus }`, assetVars{})
	require.Error(t, err)
}
