package cryptokey

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecode_ValidKey(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	keyHex := hex.EncodeToString(key)

	got, err := Decode(keyHex, "test")
	require.NoError(t, err)
	assert.Equal(t, key, got)
}

func TestDecode_InvalidHex(t *testing.T) {
	_, err := Decode("not-hex", "csrf")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "csrf")
	assert.Contains(t, err.Error(), "hex encoded")
}

func TestDecode_WrongLength(t *testing.T) {
	shortKey := hex.EncodeToString(make([]byte, 16))

	_, err := Decode(shortKey, "session hash")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session hash")
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestResolve_WithKey(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 10)
	}
	keyHex := hex.EncodeToString(key)

	got, err := Resolve(keyHex, "csrf")
	require.NoError(t, err)
	assert.Equal(t, key, got)
}

func TestResolve_EmptyGeneratesRandom(t *testing.T) {
	got, err := Resolve("", "csrf")
	require.NoError(t, err)
	assert.Len(t, got, 32)

	// Verify it's not all zeros (extremely unlikely for random).
	allZero := true
	for _, b := range got {
		if b != 0 {
			allZero = false
			break
		}
	}
	assert.False(t, allZero)
}

func TestResolve_EmptyGeneratesDifferentKeys(t *testing.T) {
	key1, err := Resolve("", "test")
	require.NoError(t, err)

	key2, err := Resolve("", "test")
	require.NoError(t, err)

	assert.NotEqual(t, key1, key2)
}

func TestResolve_InvalidKeyReturnsError(t *testing.T) {
	_, err := Resolve("invalid", "session block")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session block")
}
