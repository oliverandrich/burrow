package authtest

import (
	"testing"

	"github.com/oliverandrich/den"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oliverandrich/burrow/contrib/auth"
)

func TestNewDB(t *testing.T) {
	db := NewDB(t)
	require.NotNil(t, db)

	// Verify user collection exists by running a count query.
	count, err := den.NewQuery[auth.User[auth.EmptyProfile]](db).Count(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestCreateUserDefaults(t *testing.T) {
	db := NewDB(t)
	user := CreateUser(t, db)

	assert.NotEmpty(t, user.ID)
	assert.Contains(t, user.Username, "testuser")
	assert.Equal(t, "user", user.Role)
	assert.True(t, user.IsActive)
	assert.Nil(t, user.Email)
}

func TestCreateUserOptions(t *testing.T) {
	db := NewDB(t)
	email := "alice@example.com"
	user := CreateUser(t, db,
		WithID("custom-id-99"),
		WithUsername("alice"),
		WithEmail(email),
		WithRole("admin"),
		WithActive(false),
	)

	assert.Equal(t, "custom-id-99", user.ID)
	assert.Equal(t, "alice", user.Username)
	require.NotNil(t, user.Email)
	assert.Equal(t, "alice@example.com", *user.Email)
	assert.Equal(t, "admin", user.Role)
	assert.False(t, user.IsActive)
}

func TestCreateUserUniqueDefaults(t *testing.T) {
	db := NewDB(t)
	u1 := CreateUser(t, db)
	u2 := CreateUser(t, db)
	u3 := CreateUser(t, db)

	assert.NotEqual(t, u1.Username, u2.Username)
	assert.NotEqual(t, u2.Username, u3.Username)
	assert.NotEqual(t, u1.ID, u2.ID)
}
