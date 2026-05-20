package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserByIDWithCredentials(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
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
	repo := NewRepository[EmptyProfile](db)

	_, err := repo.GetUserByIDWithCredentials(context.Background(), "nonexistent")
	require.Error(t, err)
}

func TestDeleteExpiredEmailVerificationTokens(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
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
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
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
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "orphan")
	require.NoError(t, err)

	err = repo.DeleteUser(ctx, user.ID)
	require.NoError(t, err)

	_, err = repo.GetUserByID(ctx, user.ID)
	require.Error(t, err)
}

func TestEmailExists(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	exists, err := repo.EmailExists(ctx, "nobody@example.com")
	require.NoError(t, err)
	assert.False(t, exists)

	_, err = repo.CreateUserWithEmail(ctx, "alice@example.com")
	require.NoError(t, err)

	exists, err = repo.EmailExists(ctx, "alice@example.com")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestGetUserByEmail(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	_, err := repo.CreateUserWithEmail(ctx, "test@example.com")
	require.NoError(t, err)

	user, err := repo.GetUserByEmail(ctx, "test@example.com")
	require.NoError(t, err)
	assert.Equal(t, "test@example.com", user.Username)
}

func TestGetUserByEmailNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)

	_, err := repo.GetUserByEmail(context.Background(), "nonexistent@example.com")
	require.Error(t, err)
}

func TestUpdateUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)

	emailVal := "alice@example.com"
	user.Email = &emailVal
	err = repo.UpdateUser(ctx, user)
	require.NoError(t, err)
	assert.False(t, user.UpdatedAt.IsZero(), "UpdatedAt should be set after update")

	got, err := repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Email)
	assert.Equal(t, "alice@example.com", *got.Email)
}

func TestCreateInvite(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	invite := &Invite{
		Email:     "bob@example.com",
		TokenHash: "invite-hash-1",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Label:     "Test invite",
	}
	err := repo.CreateInvite(ctx, invite)
	require.NoError(t, err)
	assert.NotEmpty(t, invite.ID)

	got, err := repo.GetInviteByTokenHash(ctx, "invite-hash-1")
	require.NoError(t, err)
	assert.Equal(t, "bob@example.com", got.Email)
	assert.Equal(t, "Test invite", got.Label)
}

func TestDeleteInvite(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	invite := &Invite{
		Email:     "carol@example.com",
		TokenHash: "invite-hash-del",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	require.NoError(t, repo.CreateInvite(ctx, invite))

	err := repo.DeleteInvite(ctx, invite.ID)
	require.NoError(t, err)

	_, err = repo.GetInviteByTokenHash(ctx, "invite-hash-del")
	require.Error(t, err)
}

// --- Paginated query tests ---

func TestListUsersPaged(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	// Create 5 users with different roles.
	for i := range 3 {
		u, err := repo.CreateUser(ctx, fmt.Sprintf("user%d", i))
		require.NoError(t, err)
		require.NoError(t, repo.SetUserRole(ctx, u.ID, RoleUser))
	}
	for i := range 2 {
		u, err := repo.CreateUser(ctx, fmt.Sprintf("admin%d", i))
		require.NoError(t, err)
		require.NoError(t, repo.SetUserRole(ctx, u.ID, RoleAdmin))
	}

	t.Run("all users first page", func(t *testing.T) {
		pr := burrow.PageRequest{Limit: 3, Page: 1}
		users, page, err := repo.ListUsersPaged(ctx, pr, "")
		require.NoError(t, err)
		assert.Len(t, users, 3)
		assert.Equal(t, 5, page.TotalCount)
		assert.True(t, page.HasMore)
	})

	t.Run("all users second page", func(t *testing.T) {
		pr := burrow.PageRequest{Limit: 3, Page: 2}
		users, page, err := repo.ListUsersPaged(ctx, pr, "")
		require.NoError(t, err)
		assert.Len(t, users, 2)
		assert.False(t, page.HasMore)
	})

	t.Run("filter by role admin", func(t *testing.T) {
		pr := burrow.PageRequest{Limit: 10, Page: 1}
		users, page, err := repo.ListUsersPaged(ctx, pr, RoleAdmin)
		require.NoError(t, err)
		assert.Len(t, users, 2)
		assert.Equal(t, 2, page.TotalCount)
		for _, u := range users {
			assert.Equal(t, RoleAdmin, u.Role)
		}
	})

	t.Run("filter by role user", func(t *testing.T) {
		pr := burrow.PageRequest{Limit: 10, Page: 1}
		users, page, err := repo.ListUsersPaged(ctx, pr, RoleUser)
		require.NoError(t, err)
		assert.Len(t, users, 3)
		assert.Equal(t, 3, page.TotalCount)
	})

	t.Run("empty result", func(t *testing.T) {
		pr := burrow.PageRequest{Limit: 10, Page: 1}
		users, page, err := repo.ListUsersPaged(ctx, pr, "nonexistent")
		require.NoError(t, err)
		assert.Empty(t, users)
		assert.Equal(t, 0, page.TotalCount)
	})
}

func TestSearchUsers(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	alice, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)
	email := "alice@example.com"
	alice.Email = &email
	require.NoError(t, repo.UpdateUser(ctx, alice))

	_, err = repo.CreateUser(ctx, "bob")
	require.NoError(t, err)

	_, err = repo.CreateUser(ctx, "charlie")
	require.NoError(t, err)

	pr := burrow.PageRequest{Limit: 10, Page: 1}

	t.Run("search by username", func(t *testing.T) {
		users, page, err := repo.SearchUsers(ctx, "alice", pr, "")
		require.NoError(t, err)
		assert.Len(t, users, 1)
		assert.Equal(t, "alice", users[0].Username)
		assert.Equal(t, 1, page.TotalCount)
	})

	t.Run("search by email", func(t *testing.T) {
		users, _, err := repo.SearchUsers(ctx, "example.com", pr, "")
		require.NoError(t, err)
		assert.Len(t, users, 1)
		assert.Equal(t, "alice", users[0].Username)
	})

	t.Run("search with role filter", func(t *testing.T) {
		require.NoError(t, repo.SetUserRole(ctx, alice.ID, RoleAdmin))
		users, _, err := repo.SearchUsers(ctx, "ali", pr, RoleAdmin)
		require.NoError(t, err)
		assert.Len(t, users, 1)

		users, _, err = repo.SearchUsers(ctx, "ali", pr, RoleUser)
		require.NoError(t, err)
		assert.Empty(t, users)
	})

	t.Run("no results", func(t *testing.T) {
		users, page, err := repo.SearchUsers(ctx, "nonexistent", pr, "")
		require.NoError(t, err)
		assert.Empty(t, users)
		assert.Equal(t, 0, page.TotalCount)
	})
}

func TestGetInviteByID(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	invite := &Invite{
		Email:     "test@example.com",
		TokenHash: "hash-getbyid",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Label:     "Test",
	}
	require.NoError(t, repo.CreateInvite(ctx, invite))

	got, err := repo.GetInviteByID(ctx, invite.ID)
	require.NoError(t, err)
	assert.Equal(t, invite.Email, got.Email)
	assert.Equal(t, invite.Label, got.Label)

	_, err = repo.GetInviteByID(ctx, "nonexistent")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestListInvitesPaged(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	for i := range 5 {
		invite := &Invite{
			Email:     fmt.Sprintf("user%d@example.com", i),
			TokenHash: fmt.Sprintf("hash-page-%d", i),
			ExpiresAt: time.Now().Add(24 * time.Hour),
			Label:     fmt.Sprintf("Invite %d", i),
		}
		require.NoError(t, repo.CreateInvite(ctx, invite))
	}

	t.Run("first page", func(t *testing.T) {
		pr := burrow.PageRequest{Limit: 3, Page: 1}
		invites, page, err := repo.ListInvitesPaged(ctx, pr)
		require.NoError(t, err)
		assert.Len(t, invites, 3)
		assert.Equal(t, 5, page.TotalCount)
		assert.True(t, page.HasMore)
	})

	t.Run("second page", func(t *testing.T) {
		pr := burrow.PageRequest{Limit: 3, Page: 2}
		invites, page, err := repo.ListInvitesPaged(ctx, pr)
		require.NoError(t, err)
		assert.Len(t, invites, 2)
		assert.False(t, page.HasMore)
	})
}

func TestDeleteEmailVerificationToken(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)

	err = repo.CreateEmailVerificationToken(ctx, user.ID, "token-del-hash", time.Now().Add(time.Hour))
	require.NoError(t, err)

	token, err := repo.GetEmailVerificationToken(ctx, "token-del-hash")
	require.NoError(t, err)

	err = repo.DeleteEmailVerificationToken(ctx, token.ID)
	require.NoError(t, err)

	_, err = repo.GetEmailVerificationToken(ctx, "token-del-hash")
	require.Error(t, err)
}
