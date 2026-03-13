package authtest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDB(t *testing.T) {
	db := NewDB(t)
	require.NotNil(t, db)

	// Verify users table exists by running a count query.
	var count int
	err := db.NewRaw("SELECT COUNT(*) FROM users").Scan(t.Context(), &count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCreateUserDefaults(t *testing.T) {
	db := NewDB(t)
	user := CreateUser(t, db)

	assert.NotZero(t, user.ID)
	assert.Equal(t, "testuser1", user.Username)
	assert.Equal(t, "user", user.Role)
	assert.True(t, user.IsActive)
	assert.Empty(t, user.Email)
	assert.Empty(t, user.Name)
}

func TestCreateUserOptions(t *testing.T) {
	db := NewDB(t)
	email := "alice@example.com"
	user := CreateUser(t, db,
		WithID(99),
		WithUsername("alice"),
		WithEmail(email),
		WithName("Alice Smith"),
		WithRole("admin"),
		WithActive(false),
	)

	assert.Equal(t, int64(99), user.ID)
	assert.Equal(t, "alice", user.Username)
	require.NotNil(t, user.Email)
	assert.Equal(t, "alice@example.com", *user.Email)
	assert.Equal(t, "Alice Smith", user.Name)
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
