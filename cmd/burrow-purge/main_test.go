package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCLI_EndToEnd drives the full CLI path against a synthetic CSS +
// template pair in a tempdir. Verifies that the output reflects both
// the class-extraction and the rule-filtering decisions end-to-end.
func TestCLI_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	css := filepath.Join(dir, "in.css")
	tmpl := filepath.Join(dir, "page.html")
	outFile := filepath.Join(dir, "out.css")

	require.NoError(t, os.WriteFile(css, []byte(`
body { margin: 0 }
.flex { display: flex }
.unused { color: red }
@media (min-width: 640px) {
    .sm\:flex { display: flex }
    .sm\:unused { color: red }
}
`), 0o644))

	require.NoError(t, os.WriteFile(tmpl, []byte(`
<div class="flex">
    <span class="sm:flex">hi</span>
</div>
`), 0o644))

	var errBuf bytes.Buffer
	err := run(context.Background(),
		[]string{
			"--css", css,
			"--content", filepath.Join(dir, "*.html"),
			"--no-burrow-contribs",
			"--out", outFile,
		},
		&errBuf,
	)
	require.NoError(t, err)

	got, err := os.ReadFile(outFile)
	require.NoError(t, err)
	out := string(got)

	assert.Contains(t, out, "body", "tag-only rule must be kept")
	assert.Contains(t, out, ".flex", "class present in template must be kept")
	assert.NotContains(t, out, ".unused", "unused class must be dropped")
	assert.Contains(t, out, "@media", "@media wrapper with surviving inner rule must be kept")
	assert.Contains(t, out, "sm\\:flex", "surviving inner rule must be kept (escaped form)")
	assert.NotContains(t, out, "sm\\:unused", "dropped inner rule must not appear")
}

func TestCLI_RequiresOut(t *testing.T) {
	var errBuf bytes.Buffer
	err := run(context.Background(), []string{}, &errBuf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--out is required")
}

func TestCLI_KeepFlagForcesClass(t *testing.T) {
	dir := t.TempDir()
	css := filepath.Join(dir, "in.css")
	outFile := filepath.Join(dir, "out.css")

	require.NoError(t, os.WriteFile(css, []byte(`.runtime-injected { color: red }`), 0o644))

	var errBuf bytes.Buffer
	err := run(context.Background(),
		[]string{
			"--css", css,
			"--content", filepath.Join(dir, "*.html"), // matches nothing
			"--no-burrow-contribs",
			"--keep", "runtime-injected",
			"--out", outFile,
		},
		&errBuf,
	)
	require.NoError(t, err)

	got, err := os.ReadFile(outFile)
	require.NoError(t, err)
	assert.Contains(t, string(got), ".runtime-injected")
}
