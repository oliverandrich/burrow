// Package authtest provides test helpers for creating auth-migrated databases
// and test users, following the convention of net/http/httptest.
package authtest

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/burrow/internal/sqlitetest"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var userCounter atomic.Int64

// NewDB returns an in-memory SQLite *bun.DB with all auth migrations applied.
// The database is closed automatically when the test finishes.
func NewDB(t *testing.T) *bun.DB {
	t.Helper()

	db := sqlitetest.OpenDB(t)

	authApp := auth.New()
	err := burrow.RunAppMigrations(t.Context(), db, authApp.Name(), authApp.MigrationFS())
	require.NoError(t, err)

	return db
}

// UserOption configures a test user.
type UserOption func(*auth.User)

// WithID sets the user ID.
func WithID(id int64) UserOption {
	return func(u *auth.User) { u.ID = id }
}

// WithUsername sets the username.
func WithUsername(username string) UserOption {
	return func(u *auth.User) { u.Username = username }
}

// WithEmail sets the email address.
func WithEmail(email string) UserOption {
	return func(u *auth.User) { u.Email = &email }
}

// WithName sets the display name.
func WithName(name string) UserOption {
	return func(u *auth.User) { u.Name = name }
}

// WithRole sets the user role.
func WithRole(role string) UserOption {
	return func(u *auth.User) { u.Role = role }
}

// WithActive sets the active status.
func WithActive(active bool) UserOption {
	return func(u *auth.User) { u.IsActive = active }
}

// CreateUser inserts a user into the database and returns it.
// Default values: Username "testuser{N}", Role "user", IsActive true.
// Each call auto-increments a counter for unique default usernames.
func CreateUser(t *testing.T, db *bun.DB, opts ...UserOption) *auth.User {
	t.Helper()

	n := userCounter.Add(1)
	user := &auth.User{
		Username: fmt.Sprintf("testuser%d", n),
		Role:     "user",
		IsActive: true,
	}

	for _, opt := range opts {
		opt(user)
	}

	if user.ID != 0 {
		_, err := db.ExecContext(t.Context(),
			"INSERT INTO users (id, username, role, is_active, name, email) VALUES (?, ?, ?, ?, ?, ?)",
			user.ID, user.Username, user.Role, user.IsActive, user.Name, user.Email)
		require.NoError(t, err)
	} else {
		var id int64
		err := db.QueryRowContext(t.Context(),
			"INSERT INTO users (username, role, is_active, name, email) VALUES (?, ?, ?, ?, ?) RETURNING id",
			user.Username, user.Role, user.IsActive, user.Name, user.Email).Scan(&id)
		require.NoError(t, err)
		user.ID = id
	}

	return user
}
