package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path/filepath"
)

// extractBinary opens the archive served via r and returns a reader
// for the single entry whose basename matches binaryName. format is
// the archive type ("tar.gz" or "zip"), derived from the asset
// pattern's Ext field. size is required for zip readers; tar.gz
// ignores it.
//
// The returned ReadCloser must be closed by the caller.
func extractBinary(r io.ReaderAt, size int64, binaryName, format string) (io.ReadCloser, error) {
	switch format {
	case "tar.gz", "tgz":
		// archive/tar wants a stream, not a ReaderAt — wrap.
		stream := io.NewSectionReader(r, 0, size)
		return openTarGz(stream, binaryName)
	case "zip":
		return openZip(r, size, binaryName)
	default:
		return nil, fmt.Errorf("selfupdate: unsupported archive format %q (want tar.gz or zip)", format)
	}
}

func openTarGz(r io.Reader, binaryName string) (io.ReadCloser, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("selfupdate: gzip reader: %w", err)
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			_ = gz.Close()
			return nil, fmt.Errorf("selfupdate: binary %q not found in tar.gz", binaryName)
		}
		if err != nil {
			_ = gz.Close()
			return nil, fmt.Errorf("selfupdate: tar walk: %w", err)
		}
		// Only regular files qualify — refuse symlink/hardlink/dir
		// entries whose name happens to match. A symlink entry's
		// "content" is the link target, not an executable; installing
		// it would brick the binary even though the archive's
		// checksum still verifies.
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(h.Name) == binaryName {
			// Hand the caller a ReadCloser whose Close also tears
			// down the gzip wrapper.
			return &gzipBoundReader{tr: tr, gz: gz}, nil
		}
	}
}

type gzipBoundReader struct {
	tr io.Reader
	gz io.Closer
}

func (g *gzipBoundReader) Read(p []byte) (int, error) { return g.tr.Read(p) }
func (g *gzipBoundReader) Close() error               { return g.gz.Close() }

func openZip(r io.ReaderAt, size int64, binaryName string) (io.ReadCloser, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("selfupdate: zip reader: %w", err)
	}
	for _, f := range zr.File {
		// Only regular files qualify; skip directories and entries
		// without a regular-file mode bit.
		if f.FileInfo().IsDir() || !f.Mode().IsRegular() {
			continue
		}
		if filepath.Base(f.Name) == binaryName {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("selfupdate: open %q in zip: %w", f.Name, err)
			}
			return rc, nil
		}
	}
	return nil, fmt.Errorf("selfupdate: binary %q not found in zip", binaryName)
}
