package bootstrap

import (
	"io"
	"io/fs"
	"strings"
	"time"
)

// overlayFS wraps a base fs.FS and replaces the content of css.html with a
// dynamically generated template that has the CSS path baked in.
type overlayFS struct {
	base    fs.FS
	cssHTML string
}

func (o *overlayFS) Open(name string) (fs.File, error) {
	if name == "css.html" {
		return &stringFile{
			name:   "css.html",
			Reader: strings.NewReader(o.cssHTML),
			size:   int64(len(o.cssHTML)),
		}, nil
	}
	return o.base.Open(name)
}

// stringFile implements fs.File for an in-memory string.
type stringFile struct {
	*strings.Reader
	name string
	size int64
}

func (f *stringFile) Close() error               { return nil }
func (f *stringFile) Stat() (fs.FileInfo, error) { return f, nil }
func (f *stringFile) Name() string               { return f.name }
func (f *stringFile) Size() int64                { return f.size }
func (f *stringFile) Mode() fs.FileMode          { return 0o444 }
func (f *stringFile) ModTime() time.Time         { return time.Time{} }
func (f *stringFile) IsDir() bool                { return false }
func (f *stringFile) Sys() any                   { return nil }

var (
	_ fs.File     = (*stringFile)(nil)
	_ fs.FileInfo = (*stringFile)(nil)
	_ io.ReaderAt = (*stringFile)(nil)
)
