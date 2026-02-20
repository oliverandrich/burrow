package auth

import (
	"context"
	"database/sql"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codeberg.org/oliverandrich/go-webapp-template/core"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"golang.org/x/crypto/bcrypt"
)

// Compile-time interface assertions.
var (
	_ core.App             = (*App)(nil)
	_ core.Migratable      = (*App)(nil)
	_ core.Configurable    = (*App)(nil)
	_ core.HasMiddleware   = (*App)(nil)
	_ core.HasRoutes       = (*App)(nil)
	_ core.HasDependencies = (*App)(nil)
)

func TestAppName(t *testing.T) {
	app := &App{}
	assert.Equal(t, "auth", app.Name())
}

func TestAppFlags(t *testing.T) {
	app := &App{}
	flags := app.Flags()

	names := make(map[string]bool)
	for _, f := range flags {
		names[f.Names()[0]] = true
	}

	assert.True(t, names["auth-login-redirect"])
	assert.True(t, names["auth-use-email"])
	assert.True(t, names["auth-require-verification"])
	assert.True(t, names["auth-invite-only"])
	assert.True(t, names["webauthn-rp-id"])
	assert.True(t, names["webauthn-rp-display-name"])
	assert.True(t, names["webauthn-rp-origin"])
}

func TestMigrationFS(t *testing.T) {
	app := &App{}
	fsys := app.MigrationFS()
	require.NotNil(t, fsys)

	// Verify the migration file exists and is readable (MigrationFS returns the sub-FS).
	data, err := fs.ReadFile(fsys, "001_initial_schema.up.sql")
	require.NoError(t, err)
	assert.Contains(t, string(data), "CREATE TABLE IF NOT EXISTS users")
}

// --- Test helpers ---

func openTestDB(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared&_pragma=foreign_keys(1)")
	require.NoError(t, err)
	t.Cleanup(func() { sqldb.Close() })

	db := bun.NewDB(sqldb, sqlitedialect.New())

	// Run the migration manually (read from the raw embed.FS).
	data, err := fs.ReadFile(migrationFS, "migrations/001_initial_schema.up.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), string(data))
	require.NoError(t, err)

	return db
}

// --- User tests ---

func TestCreateAndGetUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "Alice")
	require.NoError(t, err)
	require.NotZero(t, user.ID)
	assert.Equal(t, "alice", user.Username)
	assert.Equal(t, "Alice", user.Name)

	got, err := repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "alice", got.Username)
}

func TestCreateUserWithEmail(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUserWithEmail(ctx, "alice@example.com", "Alice")
	require.NoError(t, err)
	require.NotNil(t, user.Email)
	assert.Equal(t, "alice@example.com", *user.Email)

	got, err := repo.GetUserByEmail(ctx, "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
}

func TestGetUserByUsername(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.CreateUser(ctx, "bob", "")
	require.NoError(t, err)

	got, err := repo.GetUserByUsername(ctx, "bob")
	require.NoError(t, err)
	assert.Equal(t, "bob", got.Username)
}

func TestGetUserNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.GetUserByID(ctx, 999)
	require.Error(t, err)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestUserExistsAndCount(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	exists, err := repo.UserExists(ctx, "alice")
	require.NoError(t, err)
	assert.False(t, exists)

	_, err = repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	exists, err = repo.UserExists(ctx, "alice")
	require.NoError(t, err)
	assert.True(t, exists)

	count, err := repo.CountUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestSetUserRole(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)
	assert.Equal(t, RoleUser, user.Role)

	err = repo.SetUserRole(ctx, user.ID, RoleAdmin)
	require.NoError(t, err)

	got, err := repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, RoleAdmin, got.Role)
	assert.True(t, got.IsAdmin())
}

func TestListUsers(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("empty database", func(t *testing.T) {
		users, err := repo.ListUsers(ctx)
		require.NoError(t, err)
		assert.Empty(t, users)
	})

	t.Run("returns all users ordered by created_at desc", func(t *testing.T) {
		_, err := repo.CreateUser(ctx, "alice", "Alice")
		require.NoError(t, err)
		_, err = repo.CreateUser(ctx, "bob", "Bob")
		require.NoError(t, err)

		users, err := repo.ListUsers(ctx)
		require.NoError(t, err)
		require.Len(t, users, 2)
		// Most recently created first.
		assert.Equal(t, "bob", users[0].Username)
		assert.Equal(t, "alice", users[1].Username)
	})
}

func TestMarkEmailVerified(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUserWithEmail(ctx, "alice@example.com", "")
	require.NoError(t, err)
	assert.False(t, user.EmailVerified)

	err = repo.MarkEmailVerified(ctx, user.ID)
	require.NoError(t, err)

	got, err := repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, got.EmailVerified)
}

// --- Credential tests ---

func TestCreateAndGetCredentials(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	cred := &Credential{
		UserID:       user.ID,
		CredentialID: []byte("cred-id-1"),
		PublicKey:    []byte("pub-key-1"),
		Name:         "My Passkey",
	}
	err = repo.CreateCredential(ctx, cred)
	require.NoError(t, err)

	creds, err := repo.GetCredentialsByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, creds, 1)
	assert.Equal(t, "My Passkey", creds[0].Name)

	count, err := repo.CountUserCredentials(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestDeleteCredential(t *testing.T) {
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
	}
	err = repo.CreateCredential(ctx, cred)
	require.NoError(t, err)

	err = repo.DeleteCredential(ctx, cred.ID, user.ID)
	require.NoError(t, err)

	count, err := repo.CountUserCredentials(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

// --- Recovery code tests ---

func TestRecoveryCodes(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	// Create bcrypt hashes for test codes.
	code := "testcode1234"
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.MinCost)
	require.NoError(t, err)

	err = repo.CreateRecoveryCodes(ctx, user.ID, []string{string(hash)})
	require.NoError(t, err)

	has, err := repo.HasRecoveryCodes(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, has)

	count, err := repo.GetUnusedRecoveryCodeCount(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Validate correct code.
	valid, err := repo.ValidateAndUseRecoveryCode(ctx, user.ID, code)
	require.NoError(t, err)
	assert.True(t, valid)

	// Code is now used.
	count, err = repo.GetUnusedRecoveryCodeCount(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Wrong code returns false.
	valid, err = repo.ValidateAndUseRecoveryCode(ctx, user.ID, "wrong")
	require.NoError(t, err)
	assert.False(t, valid)
}

func TestDeleteRecoveryCodes(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	hash, _ := bcrypt.GenerateFromPassword([]byte("code"), bcrypt.MinCost)
	err = repo.CreateRecoveryCodes(ctx, user.ID, []string{string(hash)})
	require.NoError(t, err)

	err = repo.DeleteRecoveryCodes(ctx, user.ID)
	require.NoError(t, err)

	has, err := repo.HasRecoveryCodes(ctx, user.ID)
	require.NoError(t, err)
	assert.False(t, has)
}

// --- Email verification tests ---

func TestEmailVerificationToken(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	tokenHash := "abc123hash"
	expiresAt := time.Now().Add(24 * time.Hour)
	err = repo.CreateEmailVerificationToken(ctx, user.ID, tokenHash, expiresAt)
	require.NoError(t, err)

	token, err := repo.GetEmailVerificationToken(ctx, tokenHash)
	require.NoError(t, err)
	assert.Equal(t, user.ID, token.UserID)

	err = repo.DeleteEmailVerificationToken(ctx, token.ID)
	require.NoError(t, err)

	_, err = repo.GetEmailVerificationToken(ctx, tokenHash)
	require.Error(t, err)
}

func TestDeleteUserEmailVerificationTokens(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	err = repo.CreateEmailVerificationToken(ctx, user.ID, "token1", time.Now().Add(time.Hour))
	require.NoError(t, err)
	err = repo.CreateEmailVerificationToken(ctx, user.ID, "token2", time.Now().Add(time.Hour))
	require.NoError(t, err)

	err = repo.DeleteUserEmailVerificationTokens(ctx, user.ID)
	require.NoError(t, err)

	_, err = repo.GetEmailVerificationToken(ctx, "token1")
	require.Error(t, err)
}

// --- Invite tests ---

func TestInviteCRUD(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	invite := &Invite{
		Email:     "bob@example.com",
		TokenHash: "invite-hash-1",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	err := repo.CreateInvite(ctx, invite)
	require.NoError(t, err)
	require.NotZero(t, invite.ID)

	got, err := repo.GetInviteByTokenHash(ctx, "invite-hash-1")
	require.NoError(t, err)
	assert.Equal(t, "bob@example.com", got.Email)
	assert.True(t, got.IsValid())

	invites, err := repo.ListInvites(ctx)
	require.NoError(t, err)
	require.Len(t, invites, 1)
}

func TestInviteMarkUsed(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	invite := &Invite{
		Email:     "bob@example.com",
		TokenHash: "invite-hash",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	err = repo.CreateInvite(ctx, invite)
	require.NoError(t, err)

	err = repo.MarkInviteUsed(ctx, invite.ID, user.ID)
	require.NoError(t, err)

	got, err := repo.GetInviteByTokenHash(ctx, "invite-hash")
	require.NoError(t, err)
	assert.True(t, got.IsUsed())
	assert.False(t, got.IsValid())
}

func TestInviteExpired(t *testing.T) {
	invite := &Invite{
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	assert.True(t, invite.IsExpired())
	assert.False(t, invite.IsValid())
}

func TestInviteDelete(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	invite := &Invite{
		Email:     "bob@example.com",
		TokenHash: "invite-hash",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	err := repo.CreateInvite(ctx, invite)
	require.NoError(t, err)

	err = repo.DeleteInvite(ctx, invite.ID)
	require.NoError(t, err)

	_, err = repo.GetInviteByTokenHash(ctx, "invite-hash")
	require.Error(t, err)
}

// --- Middleware tests ---

func TestAuthMiddlewareNoSession(t *testing.T) {
	app := &App{config: &Config{LoginRedirect: "/dashboard"}}

	e := echo.New()
	for _, mw := range app.Middleware() {
		e.Use(mw)
	}

	var gotUser *User
	e.GET("/test", func(c *echo.Context) error {
		gotUser = GetUser(c)
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Nil(t, gotUser)
}

func TestRequireAuthRedirects(t *testing.T) {
	e := echo.New()
	e.Use(RequireAuth())
	e.GET("/protected", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected?foo=bar", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "/auth/login?next=")
}

func TestRequireAuthAllowsAuthenticated(t *testing.T) {
	e := echo.New()
	// Inject user into context before RequireAuth.
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			SetUser(c, &User{ID: 1})
			return next(c)
		}
	})
	e.Use(RequireAuth())
	e.GET("/protected", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireAdmin(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		wantStatus int
	}{
		{"forbids non-admin", RoleUser, http.StatusForbidden},
		{"allows admin", RoleAdmin, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c *echo.Context) error {
					SetUser(c, &User{ID: 1, Role: tt.role})
					return next(c)
				}
			})
			e.Use(RequireAdmin())
			e.GET("/admin", func(c *echo.Context) error {
				return c.String(http.StatusOK, "ok")
			})

			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

// --- Helper tests ---

func TestGetUserFromContext(t *testing.T) {
	user := &User{ID: 42, Username: "alice"}
	c := echo.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	SetUser(c, user)

	got := GetUser(c)
	require.NotNil(t, got)
	assert.Equal(t, int64(42), got.ID)
	assert.True(t, IsAuthenticated(c))
}

func TestGetUserFromEmptyContext(t *testing.T) {
	c := echo.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	assert.Nil(t, GetUser(c))
	assert.False(t, IsAuthenticated(c))
}

func TestSafeRedirectPath(t *testing.T) {
	tests := []struct {
		next     string
		expected string
	}{
		{"/dashboard", "/dashboard"},
		{"/settings?tab=profile", "/settings?tab=profile"},
		{"", "/default"},
		{"https://evil.com/steal", "/default"},
		{"//evil.com", "/default"},
	}

	for _, tt := range tests {
		t.Run(tt.next, func(t *testing.T) {
			assert.Equal(t, tt.expected, SafeRedirectPath(tt.next, "/default"))
		})
	}
}

// --- Model tests ---

func TestUserWebAuthnMethods(t *testing.T) {
	user := &User{
		ID:       42,
		Username: "alice",
		Name:     "Alice Smith",
	}

	assert.Equal(t, "alice", user.WebAuthnName())
	assert.Equal(t, "Alice Smith", user.WebAuthnDisplayName())
	assert.Len(t, user.WebAuthnID(), 8)
	assert.Empty(t, user.WebAuthnIcon())
}

func TestUserWebAuthnDisplayNameFallback(t *testing.T) {
	user := &User{ID: 1, Username: "bob"}
	assert.Equal(t, "bob", user.WebAuthnDisplayName())
}

func TestInviteIsValid(t *testing.T) {
	valid := &Invite{ExpiresAt: time.Now().Add(time.Hour)}
	assert.True(t, valid.IsValid())

	expired := &Invite{ExpiresAt: time.Now().Add(-time.Hour)}
	assert.False(t, expired.IsValid())

	now := time.Now()
	used := &Invite{ExpiresAt: time.Now().Add(time.Hour), UsedAt: &now}
	assert.False(t, used.IsValid())
}
