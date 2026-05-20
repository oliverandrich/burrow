package auth

import (
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
	"github.com/oliverandrich/burrow/burrowtest"
	"github.com/oliverandrich/den"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebAuthnCredentials(t *testing.T) {
	user := &User[EmptyProfile]{
		Username: "alice",
		Credentials: []Credential{
			{
				CredentialID:    []byte("cred-1"),
				PublicKey:       []byte("key-1"),
				SignCount:       5,
				BackupEligible:  true,
				BackupState:     false,
				AttestationType: "none",
				Transports:      "usb,ble",
				AAGUID:          []byte("aaguid-1"),
			},
			{
				CredentialID: []byte("cred-2"),
				PublicKey:    []byte("key-2"),
			},
		},
	}

	creds := user.WebAuthnCredentials()
	require.Len(t, creds, 2)

	assert.Equal(t, []byte("cred-1"), creds[0].ID)
	assert.Equal(t, []byte("key-1"), creds[0].PublicKey)
	assert.Equal(t, uint32(5), creds[0].Authenticator.SignCount)
	assert.True(t, creds[0].Flags.BackupEligible)
	assert.False(t, creds[0].Flags.BackupState)
	assert.Equal(t, "none", creds[0].AttestationType)

	assert.Equal(t, []byte("cred-2"), creds[1].ID)
}

func TestWebAuthnCredentialsEmpty(t *testing.T) {
	user := &User[EmptyProfile]{}
	creds := user.WebAuthnCredentials()
	assert.Empty(t, creds)
}

func TestToWebAuthn(t *testing.T) {
	cred := &Credential{
		CredentialID:    []byte("cred-id"),
		PublicKey:       []byte("pub-key"),
		SignCount:       10,
		BackupEligible:  true,
		BackupState:     true,
		AttestationType: "packed",
		Transports:      "usb,nfc",
		AAGUID:          []byte("aaguid"),
	}

	wa := cred.ToWebAuthn()
	assert.Equal(t, []byte("cred-id"), wa.ID)
	assert.Equal(t, []byte("pub-key"), wa.PublicKey)
	assert.Equal(t, uint32(10), wa.Authenticator.SignCount)
	assert.True(t, wa.Flags.BackupEligible)
	assert.True(t, wa.Flags.BackupState)
	assert.Equal(t, "packed", wa.AttestationType)
	require.Len(t, wa.Transport, 2)
	assert.Equal(t, protocol.AuthenticatorTransport("usb"), wa.Transport[0])
	assert.Equal(t, protocol.AuthenticatorTransport("nfc"), wa.Transport[1])
}

func TestToWebAuthnNoTransports(t *testing.T) {
	cred := &Credential{
		CredentialID: []byte("cred-id"),
		PublicKey:    []byte("pub-key"),
		Transports:   "",
	}

	wa := cred.ToWebAuthn()
	assert.Empty(t, wa.Transport)
}

func TestNewCredentialFromWebAuthn(t *testing.T) {
	waCred := &gowebauthn.Credential{
		ID:              []byte("cred-id"),
		PublicKey:       []byte("pub-key"),
		AttestationType: "none",
		Transport:       []protocol.AuthenticatorTransport{"usb", "ble"},
		Flags: gowebauthn.CredentialFlags{
			BackupEligible: true,
			BackupState:    false,
		},
		Authenticator: gowebauthn.Authenticator{
			AAGUID:    []byte("aaguid"),
			SignCount: 42,
		},
	}

	cred := NewCredentialFromWebAuthn("user-99", waCred)
	assert.Equal(t, "user-99", cred.UserID)
	assert.Equal(t, []byte("cred-id"), cred.CredentialID)
	assert.Equal(t, []byte("pub-key"), cred.PublicKey)
	assert.Equal(t, "none", cred.AttestationType)
	assert.Equal(t, "usb,ble", cred.Transports)
	assert.Equal(t, "Passkey", cred.Name)
	assert.True(t, cred.BackupEligible)
	assert.False(t, cred.BackupState)
	assert.Equal(t, uint32(42), cred.SignCount)
	assert.Equal(t, []byte("aaguid"), cred.AAGUID)
}

func TestUserString(t *testing.T) {
	u := User[EmptyProfile]{Username: "bob"}
	assert.Equal(t, "bob", u.String(), "String returns Username; display-name extensions live in Profile")
}

func TestTransportsFromWebAuthn(t *testing.T) {
	result := TransportsFromWebAuthn([]protocol.AuthenticatorTransport{"usb", "nfc", "ble"})
	assert.Equal(t, "usb,nfc,ble", result)

	result = TransportsFromWebAuthn(nil)
	assert.Empty(t, result)
}

// indexedProfile is a fixture for TestProfileFieldsRespectDenTags. The
// den:"unique" tag on Slug must flow through Den's nested-struct schema
// walk and produce a real unique index on json_extract(data,
// '$.profile.slug').
type indexedProfile struct {
	Slug string `json:"slug" den:"unique"`
}

func TestProfileFieldsRespectDenTags(t *testing.T) {
	ctx := t.Context()
	db := burrowtest.DB(t)
	require.NoError(t, den.Register(ctx, db, &User[indexedProfile]{}))

	first := &User[indexedProfile]{
		Username: "alice",
		Role:     RoleUser,
		IsActive: true,
		Profile:  indexedProfile{Slug: "shared"},
	}
	require.NoError(t, den.Save(ctx, db, first))

	second := &User[indexedProfile]{
		Username: "bob",
		Role:     RoleUser,
		IsActive: true,
		Profile:  indexedProfile{Slug: "shared"},
	}
	err := den.Save(ctx, db, second)
	require.Error(t, err, "duplicate Profile.Slug must violate the nested unique index")
}
