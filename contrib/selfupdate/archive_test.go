package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractBinary_TarGz(t *testing.T) {
	want := []byte("hello world binary contents")
	buf := makeTarGz(t, []tarEntry{
		{name: "README.md", body: []byte("docs"), typ: tar.TypeReg},
		{name: "myapp", body: want, typ: tar.TypeReg},
	})

	got, err := extractBinary(bytes.NewReader(buf), int64(len(buf)), "myapp", "tar.gz")
	require.NoError(t, err)
	body, err := io.ReadAll(got)
	require.NoError(t, err)
	_ = got.Close()
	assert.Equal(t, want, body)
}

func TestExtractBinary_TarGzSkipsSymlinkEntry(t *testing.T) {
	// A malicious tar.gz with a symlink entry named the same as the
	// binary must NOT be installed — installing it would write a
	// zero-byte file (symlink content is the target string, not the
	// linked-to data). The extractor must skip and find the regular
	// file entry instead.
	want := []byte("real binary contents")
	buf := makeTarGz(t, []tarEntry{
		{name: "myapp", body: []byte("/usr/bin/sh"), typ: tar.TypeSymlink, linkname: "/usr/bin/sh"},
		{name: "deeper/myapp", body: want, typ: tar.TypeReg},
	})

	got, err := extractBinary(bytes.NewReader(buf), int64(len(buf)), "myapp", "tar.gz")
	require.NoError(t, err)
	body, err := io.ReadAll(got)
	require.NoError(t, err)
	_ = got.Close()
	assert.Equal(t, want, body, "regular-file entry must be preferred over symlink")
}

func TestExtractBinary_TarGzAllSymlinks(t *testing.T) {
	buf := makeTarGz(t, []tarEntry{
		{name: "myapp", typ: tar.TypeSymlink, linkname: "/usr/bin/sh"},
	})
	_, err := extractBinary(bytes.NewReader(buf), int64(len(buf)), "myapp", "tar.gz")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "myapp")
}

func TestExtractBinary_Zip(t *testing.T) {
	want := []byte("hello windows binary")
	buf := makeZip(t, map[string][]byte{
		"README.md": []byte("docs"),
		"myapp.exe": want,
	})

	bufBytes := buf.Bytes()
	got, err := extractBinary(bytes.NewReader(bufBytes), int64(len(bufBytes)), "myapp.exe", "zip")
	require.NoError(t, err)
	body, err := io.ReadAll(got)
	require.NoError(t, err)
	_ = got.Close()
	assert.Equal(t, want, body)
}

func TestExtractBinary_BinaryNotInArchive(t *testing.T) {
	buf := makeTarGz(t, []tarEntry{{name: "README.md", body: []byte("docs"), typ: tar.TypeReg}})
	_, err := extractBinary(bytes.NewReader(buf), int64(len(buf)), "myapp", "tar.gz")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "myapp")
}

func TestExtractBinary_UnsupportedFormat(t *testing.T) {
	_, err := extractBinary(bytes.NewReader(nil), 0, "myapp", "rar")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rar")
}

type tarEntry struct {
	name     string
	body     []byte
	typ      byte
	linkname string
}

func makeTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{
			Name:     e.name,
			Mode:     0o644,
			Size:     int64(len(e.body)),
			Typeflag: e.typ,
			Linkname: e.linkname,
		}
		if e.typ == tar.TypeSymlink {
			h.Size = 0
		}
		require.NoError(t, tw.WriteHeader(h))
		if h.Size > 0 {
			_, err := tw.Write(e.body)
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func makeZip(t *testing.T, files map[string][]byte) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := zw.Create(name)
		require.NoError(t, err)
		_, err = f.Write(body)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return &buf
}
