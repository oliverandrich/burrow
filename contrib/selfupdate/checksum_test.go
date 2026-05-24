package selfupdate

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseChecksums(t *testing.T) {
	in := strings.NewReader(`
# comment
8b1a9953c4611296a827abf8c47804d7  empty-test
a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3  myapp-1.2.3-linux-x86_64.tar.gz
deadbeefcafe  bogus
`)
	got, err := parseChecksums(in)
	require.NoError(t, err)

	assert.Equal(t, "8b1a9953c4611296a827abf8c47804d7", got["empty-test"])
	assert.Equal(t,
		"a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3",
		got["myapp-1.2.3-linux-x86_64.tar.gz"])
}

func TestChecksumFor_HitAndMiss(t *testing.T) {
	sums := map[string]string{
		"a.tgz": "a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3",
	}
	got, err := checksumFor(sums, "a.tgz")
	require.NoError(t, err)
	assert.Len(t, got, 32, "32 bytes = 256 bits SHA256")

	_, err = checksumFor(sums, "b.tgz")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "b.tgz")
}

func TestChecksumFor_RejectsBadHex(t *testing.T) {
	sums := map[string]string{"a.tgz": "not-hex"}
	_, err := checksumFor(sums, "a.tgz")
	require.Error(t, err)
}

func TestParseChecksums_RejectsWrongLength(t *testing.T) {
	in := strings.NewReader("deadbeef  short\n")
	got, err := parseChecksums(in)
	require.NoError(t, err)
	// "short" record IS kept (raw hex) — validation happens in checksumFor.
	assert.Equal(t, "deadbeef", got["short"])
	_, err = checksumFor(got, "short")
	require.Error(t, err)
}
