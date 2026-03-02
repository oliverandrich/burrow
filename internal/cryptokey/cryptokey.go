// Package cryptokey provides shared utilities for resolving and decoding
// hex-encoded 32-byte cryptographic keys used by contrib apps (csrf, session).
package cryptokey

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
)

// Resolve returns a 32-byte key from keyHex. If keyHex is empty, a random key
// is generated and a warning is logged. keyType is used in log and error
// messages (e.g. "csrf", "session hash").
func Resolve(keyHex, keyType string) ([]byte, error) {
	if keyHex != "" {
		return Decode(keyHex, keyType)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate %s key: %w", keyType, err)
	}
	slog.Warn("No "+keyType+" key configured, using random key (will not persist across restarts)",
		"generated_key", hex.EncodeToString(key),
	)
	return key, nil
}

// Decode decodes a hex-encoded string and validates it is exactly 32 bytes.
// keyType is used in error messages (e.g. "csrf", "session block").
func Decode(keyHex, keyType string) ([]byte, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid %s key: must be hex encoded", keyType)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid %s key: must be 32 bytes", keyType)
	}
	return key, nil
}
