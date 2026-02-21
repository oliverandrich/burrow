// Package csrf provides CSRF protection as a burrow contrib app.
// It wraps gorilla/csrf and provides context helpers for reading
// the CSRF token in templates.
package csrf

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
)

// ctxKeyCSRFToken is the context key for the CSRF token.
type ctxKeyCSRFToken struct{}

// WithToken stores a CSRF token in the context.
func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, ctxKeyCSRFToken{}, token)
}

// Token retrieves the CSRF token from the context.
func Token(ctx context.Context) string {
	if token, ok := ctx.Value(ctxKeyCSRFToken{}).(string); ok {
		return token
	}
	return ""
}

// --- Key utilities ---

func resolveKey(keyHex string) ([]byte, error) {
	if keyHex != "" {
		return decodeKey(keyHex)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, errors.New("csrf: failed to generate auth key")
	}
	slog.Warn("No CSRF key configured, using random key (tokens will not persist across restarts)",
		"generated_key", hex.EncodeToString(key),
	)
	return key, nil
}

func decodeKey(keyHex string) ([]byte, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, errors.New("csrf: invalid key: must be hex encoded")
	}
	if len(key) != 32 {
		return nil, errors.New("csrf: invalid key: must be 32 bytes")
	}
	return key, nil
}
