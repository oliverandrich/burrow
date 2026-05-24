package selfupdate

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// parseChecksums reads a goreleaser-format `checksums.txt`:
//
//	<hex-sha256>  <filename>
//
// per line. Lines starting with `#` and blank lines are ignored.
// The map is keyed by filename, value is the raw hex string —
// checksumFor converts to bytes and validates length per lookup.
func parseChecksums(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// goreleaser writes "<hex>  <name>" (two spaces); accept any
		// whitespace separator to be forgiving.
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		out[fields[1]] = fields[0]
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("selfupdate: read checksums: %w", err)
	}
	return out, nil
}

// checksumFor looks up name in sums and returns the raw 32-byte
// SHA256 digest. Returns an error when the name is missing or the
// stored value is not a 64-hex-char SHA256.
func checksumFor(sums map[string]string, name string) ([]byte, error) {
	hexSum, ok := sums[name]
	if !ok {
		return nil, fmt.Errorf("selfupdate: no checksum for %q in checksums file", name)
	}
	raw, err := hex.DecodeString(hexSum)
	if err != nil {
		return nil, fmt.Errorf("selfupdate: checksum for %q is not valid hex: %w", name, err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("selfupdate: checksum for %q is %d bytes, want 32 (SHA256)", name, len(raw))
	}
	return raw, nil
}
