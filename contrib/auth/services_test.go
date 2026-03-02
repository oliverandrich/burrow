package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// --- Recovery service tests ---

func TestRecoveryServiceGenerateCodes(t *testing.T) {
	svc := NewRecoveryService()

	codes, hashes, err := svc.GenerateCodes(8)
	require.NoError(t, err)
	require.Len(t, codes, 8)
	require.Len(t, hashes, 8)

	for _, code := range codes {
		parts := strings.Split(code, "-")
		require.Len(t, parts, 3, "code %q should have 3 parts", code)
		for _, p := range parts {
			assert.Len(t, p, 4, "each part should be 4 characters")
		}
	}

	for i, code := range codes {
		normalized := NormalizeCode(code)
		err := bcrypt.CompareHashAndPassword([]byte(hashes[i]), []byte(normalized))
		assert.NoError(t, err, "code should match its hash")
	}
}

func TestRecoveryServiceDefaultCount(t *testing.T) {
	svc := NewRecoveryService()
	codes, _, err := svc.GenerateCodes(0)
	require.NoError(t, err)
	assert.Len(t, codes, CodeCount)
}

func TestNormalizeCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ABCD-EFGH-IJKL", "abcdefghijkl"},
		{"abcd-efgh-ijkl", "abcdefghijkl"},
		{"abcdefghijkl", "abcdefghijkl"},
		{"AB-CD", "abcd"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, NormalizeCode(tt.input))
	}
}

// --- Token utility tests ---

func TestHashToken(t *testing.T) {
	token := "test-token-123"
	hash := HashToken(token)

	assert.Len(t, hash, 64)
	assert.Equal(t, hash, HashToken(token), "should be deterministic")
	assert.NotEqual(t, hash, HashToken("other-token"))

	expected := sha256.Sum256([]byte(token))
	assert.Equal(t, hex.EncodeToString(expected[:]), hash)
}

func TestGenerateToken(t *testing.T) {
	plain, hash, expiresAt, err := GenerateToken()
	require.NoError(t, err)

	assert.Len(t, plain, 64)
	assert.Equal(t, HashToken(plain), hash)
	assert.True(t, expiresAt.After(time.Now().Add(23*time.Hour)))
	assert.True(t, expiresAt.Before(time.Now().Add(25*time.Hour)))

	plain2, _, _, _ := GenerateToken()
	assert.NotEqual(t, plain, plain2)
}

func TestGenerateInviteToken(t *testing.T) {
	plain, hash, err := GenerateInviteToken()
	require.NoError(t, err)

	assert.Len(t, plain, 64)
	assert.Equal(t, HashToken(plain), hash)
}

// --- WebAuthn service tests ---

func TestNewWebAuthnService(t *testing.T) {
	svc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)
	require.NotNil(t, svc)
	assert.NotNil(t, svc.WebAuthn())
}

func TestWebAuthnServiceRegistrationSession(t *testing.T) {
	svc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	data := &gowebauthn.SessionData{Challenge: "test-challenge"}
	svc.StoreRegistrationSession(42, data)

	got, err := svc.GetRegistrationSession(42)
	require.NoError(t, err)
	assert.Equal(t, "test-challenge", got.Challenge)

	// Second get should fail (one-time use).
	_, err = svc.GetRegistrationSession(42)
	require.Error(t, err)
}

func TestWebAuthnServiceDiscoverableSession(t *testing.T) {
	svc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	data := &gowebauthn.SessionData{Challenge: "disco-challenge"}
	svc.StoreDiscoverableSession("session-123", data)

	got, err := svc.GetDiscoverableSession("session-123")
	require.NoError(t, err)
	assert.Equal(t, "disco-challenge", got.Challenge)

	_, err = svc.GetDiscoverableSession("session-123")
	require.Error(t, err)
}

func TestWebAuthnServiceNotFound(t *testing.T) {
	svc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	_, err = svc.GetRegistrationSession(999)
	require.Error(t, err)

	_, err = svc.GetDiscoverableSession("nonexistent")
	require.Error(t, err)
}

func TestWebAuthnServiceCleanupStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	svc, err := NewWebAuthnService(ctx, "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	ws := svc.(*webauthnService)

	// Cancel the context — cleanup goroutine should exit.
	cancel()

	select {
	case <-ws.done:
		// OK, goroutine exited.
	case <-time.After(time.Second):
		t.Fatal("cleanup goroutine did not stop within 1 second")
	}
}
