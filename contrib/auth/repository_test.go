package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserByIDWithCredentials(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "Alice")
	require.NoError(t, err)

	cred := &Credential{
		UserID:       user.ID,
		CredentialID: []byte("cred-1"),
		PublicKey:    []byte("key-1"),
		Name:         "My Passkey",
	}
	require.NoError(t, repo.CreateCredential(ctx, cred))

	got, err := repo.GetUserByIDWithCredentials(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "alice", got.Username)
	require.Len(t, got.Credentials, 1)
	assert.Equal(t, "My Passkey", got.Credentials[0].Name)
}

func TestGetUserByIDWithCredentialsNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	_, err := repo.GetUserByIDWithCredentials(context.Background(), 999)
	require.Error(t, err)
}

func TestDeleteExpiredEmailVerificationTokens(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	// Create an expired token.
	err = repo.CreateEmailVerificationToken(ctx, user.ID, "expired-hash", time.Now().Add(-time.Hour))
	require.NoError(t, err)

	// Create a valid token.
	err = repo.CreateEmailVerificationToken(ctx, user.ID, "valid-hash", time.Now().Add(time.Hour))
	require.NoError(t, err)

	// Delete expired tokens.
	err = repo.DeleteExpiredEmailVerificationTokens(ctx)
	require.NoError(t, err)

	// Expired token should be gone.
	_, err = repo.GetEmailVerificationToken(ctx, "expired-hash")
	require.Error(t, err)

	// Valid token should still exist.
	token, err := repo.GetEmailVerificationToken(ctx, "valid-hash")
	require.NoError(t, err)
	assert.Equal(t, user.ID, token.UserID)
}

func TestUpdateCredentialSignCount(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	cred := &Credential{
		UserID:       user.ID,
		CredentialID: []byte("cred-id-1"),
		PublicKey:    []byte("pub-key-1"),
		Name:         "Passkey",
		SignCount:    0,
	}
	require.NoError(t, repo.CreateCredential(ctx, cred))

	// Update sign count.
	err = repo.UpdateCredentialSignCount(ctx, []byte("cred-id-1"), 42)
	require.NoError(t, err)

	// Verify the sign count was updated.
	creds, err := repo.GetCredentialsByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, creds, 1)
	assert.Equal(t, uint32(42), creds[0].SignCount)
}

func TestDeleteUserByID(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "orphan", "")
	require.NoError(t, err)

	err = repo.DeleteUser(ctx, user.ID)
	require.NoError(t, err)

	_, err = repo.GetUserByID(ctx, user.ID)
	require.Error(t, err)
}

func TestEmailExists(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	exists, err := repo.EmailExists(ctx, "nobody@example.com")
	require.NoError(t, err)
	assert.False(t, exists)

	_, err = repo.CreateUserWithEmail(ctx, "alice@example.com", "Alice")
	require.NoError(t, err)

	exists, err = repo.EmailExists(ctx, "alice@example.com")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestGetUserByEmail(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.CreateUserWithEmail(ctx, "test@example.com", "Test")
	require.NoError(t, err)

	user, err := repo.GetUserByEmail(ctx, "test@example.com")
	require.NoError(t, err)
	assert.Equal(t, "test@example.com", user.Username)
}

func TestGetUserByEmailNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	_, err := repo.GetUserByEmail(context.Background(), "nonexistent@example.com")
	require.Error(t, err)
}
