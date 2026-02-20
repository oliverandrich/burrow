// Package staticfiles provides static file serving as a burrow contrib app.
// It computes content hashes at startup and serves files under hashed URLs
// for cache busting, similar to Django's ManifestStaticFilesStorage.
package staticfiles

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"strings"
)

type ctxKeyApp struct{}

// URL returns the hashed URL for a static file. It reads the App from context
// (injected by the middleware) to resolve the content-hashed path.
// If no App is in context, it returns the name as-is (safe fallback).
func URL(ctx context.Context, name string) string {
	a, ok := ctx.Value(ctxKeyApp{}).(*App)
	if !ok || a == nil {
		return name
	}
	if hashed, exists := a.manifest[name]; exists {
		return a.prefix + hashed
	}
	return a.prefix + name
}

// --- Content hashing ---

// contentHash computes a SHA-256 hash of data and returns the first 8 hex chars.
func contentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:8]
}

// hashedName inserts a hash before the last extension in a file path.
// "dist/styles.css" + "a1b2c3d4" → "dist/styles.a1b2c3d4.css"
func hashedName(path, hash string) string {
	idx := strings.LastIndex(path, ".")
	if idx < 0 {
		return path + "." + hash
	}
	return path[:idx] + "." + hash + path[idx:]
}

// buildManifest walks the FS, computes content hashes, and returns two maps:
// manifest maps original paths to hashed paths, files maps hashed paths back to originals.
func buildManifest(fsys fs.FS) (manifest map[string]string, files map[string]string) {
	manifest = make(map[string]string)
	files = make(map[string]string)

	_ = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		data, readErr := readFile(fsys, path)
		if readErr != nil {
			return readErr
		}

		hash := contentHash(data)
		hashed := hashedName(path, hash)
		manifest[path] = hashed
		files[hashed] = path

		return nil
	})

	return manifest, files
}

func readFile(fsys fs.FS, path string) ([]byte, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

// --- hashedFS: translates hashed filenames to originals ---

// hashedFS wraps an fs.FS to serve files by their hashed names.
// It looks up the original filename in the reverse map, falling back
// to direct FS access for files not in the map (e.g. externally-hashed fonts).
type hashedFS struct {
	fsys  fs.FS
	files map[string]string // hashed path → original path
}

func (h *hashedFS) Open(name string) (fs.File, error) {
	if original, ok := h.files[name]; ok {
		return h.fsys.Open(original)
	}
	return h.fsys.Open(name)
}

// isHashedAsset checks if the path contains a content hash pattern (name.XXXXXXXX.ext).
func isHashedAsset(path string) bool {
	parts := strings.Split(path, ".")
	if len(parts) < 3 {
		return false
	}
	hash := parts[len(parts)-2]
	if len(hash) != 8 {
		return false
	}
	for _, c := range hash {
		isDigit := c >= '0' && c <= '9'
		isLower := c >= 'a' && c <= 'z'
		isUpper := c >= 'A' && c <= 'Z'
		if !isDigit && !isLower && !isUpper {
			return false
		}
	}
	return true
}
