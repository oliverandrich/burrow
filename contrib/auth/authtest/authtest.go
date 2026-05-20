// Package authtest provides test helpers for creating auth-migrated databases
// and test users, following the convention of net/http/httptest.
package authtest

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/oliverandrich/burrow/burrowtest"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/den"
	_ "github.com/oliverandrich/den/backend/sqlite" // register sqlite:// scheme for den.OpenURL — explicit so authtest's deps are visible at its own file even if burrowtest's import surface changes
	"github.com/stretchr/testify/require"
)

var userCounter atomic.Int64

// NewDB returns a Den *den.DB with all auth document types registered.
// Struct-tag validation is always-on at the Den layer. The database is
// closed automatically when the test finishes.
func NewDB(t *testing.T) *den.DB {
	t.Helper()

	db, err := den.OpenURL(t.Context(), burrowtest.TempDSN(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	authApp := auth.New[auth.EmptyProfile]()
	for _, doc := range authApp.Documents() {
		err := den.Register(context.Background(), db, doc)
		require.NoError(t, err)
	}

	return db
}

// UserOption configures a test user. authtest hard-codes
// [auth.EmptyProfile] — apps with a custom Profile use their own seeding
// helpers since Profile fields are app-specific.
type UserOption func(*auth.User[auth.EmptyProfile])

// WithID sets the user ID.
func WithID(id string) UserOption {
	return func(u *auth.User[auth.EmptyProfile]) { u.ID = id }
}

// WithUsername sets the username.
func WithUsername(username string) UserOption {
	return func(u *auth.User[auth.EmptyProfile]) { u.Username = username }
}

// WithEmail sets the email address.
func WithEmail(email string) UserOption {
	return func(u *auth.User[auth.EmptyProfile]) { u.Email = &email }
}

// WithRole sets the user role.
func WithRole(role string) UserOption {
	return func(u *auth.User[auth.EmptyProfile]) { u.Role = role }
}

// WithActive sets the active status.
func WithActive(active bool) UserOption {
	return func(u *auth.User[auth.EmptyProfile]) { u.IsActive = active }
}

// CreateUser inserts a user into the database and returns it.
// Default values: Username "testuser{N}", Role "user", IsActive true.
// Each call auto-increments a counter for unique default usernames.
func CreateUser(t *testing.T, db *den.DB, opts ...UserOption) *auth.User[auth.EmptyProfile] {
	t.Helper()

	n := userCounter.Add(1)
	user := &auth.User[auth.EmptyProfile]{
		Username: fmt.Sprintf("testuser%d", n),
		Role:     "user",
		IsActive: true,
	}

	for _, opt := range opts {
		opt(user)
	}

	err := den.Save(context.Background(), db, user)
	require.NoError(t, err)

	return user
}
