// Package authtest provides test helpers for creating auth-migrated databases
// and test users, following the convention of net/http/httptest.
package authtest

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/den"
	_ "github.com/oliverandrich/den/backend/sqlite" // register sqlite:// scheme for OpenURL
	"github.com/oliverandrich/den/validate"
	"github.com/stretchr/testify/require"
)

var userCounter atomic.Int64

// NewDB returns a Den *den.DB with all auth document types registered.
// Struct-tag validation is enabled by default to match [burrow.OpenDB].
// The database is closed automatically when the test finishes.
func NewDB(t *testing.T) *den.DB {
	t.Helper()

	dsn := "sqlite:///" + filepath.Join(t.TempDir(), "auth_test.db")
	db, err := den.OpenURL(t.Context(), dsn, validate.WithValidation())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	authApp := auth.New()
	for _, doc := range authApp.Documents() {
		err := den.Register(context.Background(), db, doc)
		require.NoError(t, err)
	}

	return db
}

// UserOption configures a test user.
type UserOption func(*auth.User)

// WithID sets the user ID.
func WithID(id string) UserOption {
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
func CreateUser(t *testing.T, db *den.DB, opts ...UserOption) *auth.User {
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

	err := den.Insert(context.Background(), db, user)
	require.NoError(t, err)

	return user
}
