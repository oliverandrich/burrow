package staticfiles

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentHash(t *testing.T) {
	hash := contentHash([]byte("body{}"))

	// Must be exactly 8 hex characters.
	assert.Len(t, hash, 8)
	for _, c := range hash {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"expected hex char, got %c", c)
	}

	// Deterministic: same input produces same hash.
	assert.Equal(t, hash, contentHash([]byte("body{}")))

	// Different content produces different hash.
	assert.NotEqual(t, hash, contentHash([]byte("other")))
}

func TestHashedName(t *testing.T) {
	tests := []struct {
		path string
		hash string
		want string
	}{
		{"dist/styles.css", "a1b2c3d4", "dist/styles.a1b2c3d4.css"},
		{"dist/app.js", "deadbeef", "dist/app.deadbeef.js"},
		{"dist/styles.min.css", "abcd1234", "dist/styles.min.abcd1234.css"},
		{"noext", "12345678", "noext.12345678"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, hashedName(tt.path, tt.hash))
		})
	}
}

func TestBuildManifest(t *testing.T) {
	fsys := fstest.MapFS{
		"dist/styles.css": &fstest.MapFile{Data: []byte("body{}")},
		"dist/app.js":     &fstest.MapFile{Data: []byte("alert(1)")},
	}

	manifest, files := buildManifest(fsys)

	// manifest maps original → hashed
	stylesHash := contentHash([]byte("body{}"))
	appHash := contentHash([]byte("alert(1)"))

	assert.Equal(t, "dist/styles."+stylesHash+".css", manifest["dist/styles.css"])
	assert.Equal(t, "dist/app."+appHash+".js", manifest["dist/app.js"])

	// files is the reverse mapping
	assert.Equal(t, "dist/styles.css", files["dist/styles."+stylesHash+".css"])
	assert.Equal(t, "dist/app.js", files["dist/app."+appHash+".js"])
}

func TestBuildManifestDeterministic(t *testing.T) {
	fsys := fstest.MapFS{
		"dist/styles.css": &fstest.MapFile{Data: []byte("body{}")},
	}

	m1, _ := buildManifest(fsys)
	m2, _ := buildManifest(fsys)

	assert.Equal(t, m1, m2)
}

func TestHashedFSOpen(t *testing.T) {
	fsys := fstest.MapFS{
		"dist/styles.css": &fstest.MapFile{Data: []byte("body{}")},
	}

	hash := contentHash([]byte("body{}"))
	files := map[string]string{
		"dist/styles." + hash + ".css": "dist/styles.css",
	}

	hfs := &hashedFS{fsys: fsys, files: files}

	// Opening via hashed name should work.
	f, err := hfs.Open("dist/styles." + hash + ".css")
	require.NoError(t, err)
	f.Close()
}

func TestHashedFSFallback(t *testing.T) {
	fsys := fstest.MapFS{
		"dist/bootstrap-icons-CVBWLLHT.woff2": &fstest.MapFile{Data: []byte("font")},
	}

	hfs := &hashedFS{fsys: fsys, files: map[string]string{}}

	// Not in reverse map — should fall through to underlying FS.
	f, err := hfs.Open("dist/bootstrap-icons-CVBWLLHT.woff2")
	require.NoError(t, err)
	f.Close()
}

func TestHashedFSNotFound(t *testing.T) {
	fsys := fstest.MapFS{}
	hfs := &hashedFS{fsys: fsys, files: map[string]string{}}

	_, err := hfs.Open("dist/nonexistent.css")
	assert.Error(t, err)
}

func TestIsHashedAsset(t *testing.T) {
	tests := []struct {
		path   string
		hashed bool
	}{
		{"/static/dist/styles.abcd1234.css", true},
		{"/static/dist/app.EFGH5678.js", true},
		{"/static/dist/icons.A1B2C3D4.woff2", true},
		{"/static/dist/styles.css", false},
		{"/static/dist/app.js", false},
		{"/test", false},
		{"/static/dist/styles.short.css", false},     // hash too short
		{"/static/dist/styles.toolongha.css", false}, // hash too long
		{"/static/dist/styles.abcd-234.css", false},  // non-alphanumeric
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.hashed, isHashedAsset(tt.path))
		})
	}
}
