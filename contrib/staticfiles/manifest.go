package staticfiles

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"strings"
)

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
func buildManifest(fsys fs.FS) (map[string]string, map[string]string, error) {
	manifest := make(map[string]string)
	files := make(map[string]string)

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, walkErr error) error {
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

	return manifest, files, err
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

// contribSource represents a contributed FS from a HasStaticFiles app.
type contribSource struct {
	fsys   fs.FS             // the contributed filesystem
	files  map[string]string // hashed path → original path (unprefixed)
	prefix string            // path prefix, e.g. "admin"
}

// hashedFS wraps an fs.FS to serve files by their hashed names.
// It looks up the original filename in the reverse map, falling back
// to direct FS access for files not in the map (e.g. externally-hashed fonts).
// It also supports contrib sources with prefixed paths.
type hashedFS struct {
	fsys     fs.FS
	files    map[string]string // hashed path → original path
	contribs []contribSource
}

func (h *hashedFS) Open(name string) (fs.File, error) {
	// Check primary FS reverse map.
	if original, ok := h.files[name]; ok {
		return h.fsys.Open(original)
	}

	// Check contrib FSes by prefix.
	for _, c := range h.contribs {
		if unprefixed, ok := strings.CutPrefix(name, c.prefix+"/"); ok {
			if original, ok := c.files[unprefixed]; ok {
				return c.fsys.Open(original)
			}
			// Fallback: try direct open (e.g. externally-hashed fonts).
			f, err := c.fsys.Open(unprefixed)
			if err == nil {
				return f, nil
			}
		}
	}

	// Fallback: direct open on primary FS.
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
