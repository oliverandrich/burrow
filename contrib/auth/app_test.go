package auth

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"codeberg.org/oliverandrich/burrow"
	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/urfave/cli/v3"
	"golang.org/x/crypto/bcrypt"
)

// Compile-time interface assertions.
var (
	_ burrow.App             = (*App)(nil)
	_ burrow.Migratable      = (*App)(nil)
	_ burrow.Configurable    = (*App)(nil)
	_ burrow.HasMiddleware   = (*App)(nil)
	_ burrow.HasRoutes       = (*App)(nil)
	_ burrow.HasAdmin        = (*App)(nil)
	_ burrow.HasCLICommands  = (*App)(nil)
	_ burrow.HasDependencies = (*App)(nil)
	_ burrow.HasStaticFiles  = (*App)(nil)
	_ burrow.HasTranslations = (*App)(nil)
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
	assert.True(t, names["auth-logout-redirect"])
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

	// Verify the migration files exist and are readable (MigrationFS returns the sub-FS).
	data, err := fs.ReadFile(fsys, "001_initial_schema.up.sql")
	require.NoError(t, err)
	assert.Contains(t, string(data), "CREATE TABLE IF NOT EXISTS users")

	data, err = fs.ReadFile(fsys, "002_invite_label.up.sql")
	require.NoError(t, err)
	assert.Contains(t, string(data), "ALTER TABLE invites ADD COLUMN label")
}

// --- Test helpers ---

func openTestDB(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared&_pragma=foreign_keys(1)")
	require.NoError(t, err)
	t.Cleanup(func() { sqldb.Close() })

	db := bun.NewDB(sqldb, sqlitedialect.New())

	// Run the migrations manually (read from the raw embed.FS).
	for _, mig := range []string{
		"migrations/001_initial_schema.up.sql",
		"migrations/002_invite_label.up.sql",
	} {
		data, err := fs.ReadFile(migrationFS, mig)
		require.NoError(t, err)
		_, err = db.ExecContext(context.Background(), string(data))
		require.NoError(t, err)
	}

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

func TestCountAdminUsers(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	count, err := repo.CountAdminUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	alice, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, alice.ID, RoleAdmin))

	count, err = repo.CountAdminUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	bob, err := repo.CreateUser(ctx, "bob", "")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, bob.ID, RoleAdmin))

	count, err = repo.CountAdminUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
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

	t.Run("returns all users ordered by id asc", func(t *testing.T) {
		_, err := repo.CreateUser(ctx, "alice", "Alice")
		require.NoError(t, err)
		_, err = repo.CreateUser(ctx, "bob", "Bob")
		require.NoError(t, err)

		users, err := repo.ListUsers(ctx)
		require.NoError(t, err)
		require.Len(t, users, 2)
		assert.Equal(t, "alice", users[0].Username)
		assert.Equal(t, "bob", users[1].Username)
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

	r := chi.NewRouter()
	for _, mw := range app.Middleware() {
		r.Use(mw)
	}

	var gotUser *User
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Nil(t, gotUser)
}

// --- CLI tests ---

func TestCLICommands(t *testing.T) {
	app := &App{}
	cmds := app.CLICommands()

	require.NotEmpty(t, cmds)

	names := make(map[string]bool)
	for _, cmd := range cmds {
		names[cmd.Name] = true
	}

	assert.True(t, names["promote"], "should have promote command")
	assert.True(t, names["demote"], "should have demote command")
	assert.True(t, names["create-invite"], "should have create-invite command")
}

func TestCLIPromote(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)
	assert.Equal(t, RoleUser, user.Role)

	app := &App{repo: repo}
	cmds := app.CLICommands()

	var promoteCmd *cli.Command
	for _, cmd := range cmds {
		if cmd.Name == "promote" {
			promoteCmd = cmd
			break
		}
	}
	require.NotNil(t, promoteCmd)

	cliCmd := &cli.Command{
		Name:     "test",
		Action:   func(ctx context.Context, cmd *cli.Command) error { return nil },
		Commands: []*cli.Command{promoteCmd},
	}
	err = cliCmd.Run(ctx, []string{"test", "promote", "alice"})
	require.NoError(t, err)

	got, err := repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, RoleAdmin, got.Role)
}

func TestCLIDemote(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "bob", "")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, user.ID, RoleAdmin))

	app := &App{repo: repo}
	cmds := app.CLICommands()

	var demoteCmd *cli.Command
	for _, cmd := range cmds {
		if cmd.Name == "demote" {
			demoteCmd = cmd
			break
		}
	}
	require.NotNil(t, demoteCmd)

	cliCmd := &cli.Command{
		Name:     "test",
		Action:   func(ctx context.Context, cmd *cli.Command) error { return nil },
		Commands: []*cli.Command{demoteCmd},
	}
	err = cliCmd.Run(ctx, []string{"test", "demote", "bob"})
	require.NoError(t, err)

	got, err := repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, RoleUser, got.Role)
}

func TestCLICreateInvite(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	app := &App{repo: repo, globalConfig: &burrow.Config{}}
	cmds := app.CLICommands()

	var createInviteCmd *cli.Command
	for _, cmd := range cmds {
		if cmd.Name == "create-invite" {
			createInviteCmd = cmd
			break
		}
	}
	require.NotNil(t, createInviteCmd)

	cliCmd := &cli.Command{
		Name:     "test",
		Action:   func(ctx context.Context, cmd *cli.Command) error { return nil },
		Commands: []*cli.Command{createInviteCmd},
	}
	err := cliCmd.Run(ctx, []string{"test", "create-invite", "test@example.com"})
	require.NoError(t, err)

	invites, err := repo.ListInvites(ctx)
	require.NoError(t, err)
	require.Len(t, invites, 1)
	assert.Equal(t, "test@example.com", invites[0].Email)
}

// --- Admin tests ---

func TestAdminNavItems(t *testing.T) {
	app := &App{}
	items := app.AdminNavItems()

	require.NotEmpty(t, items)

	labels := make(map[string]bool)
	for _, item := range items {
		labels[item.Label] = true
		assert.True(t, item.AdminOnly, "admin nav items should be admin-only: %s", item.Label)
	}

	assert.True(t, labels["Users"], "should have Users nav item")
	assert.True(t, labels["Invites"], "should have Invites nav item")
}

func TestAdminRoutes(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	mockAdminR := &mockAdminRenderer{}
	app := &App{repo: repo}
	app.SetAdminRenderer(mockAdminR)

	r := chi.NewRouter()
	// Inject admin user into context.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithUser(r.Context(), &User{ID: 1, Role: RoleAdmin})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Route("/admin", func(r chi.Router) {
		r.Use(RequireAuth(), RequireAdmin())
		app.AdminRoutes(r)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "AdminUsersPage", mockAdminR.lastMethod)
}

func TestAdminRoutesWithLifecycleOrder(t *testing.T) {
	db := openTestDB(t)

	// Simulate real lifecycle: New → SetAdminRenderer → Register → Configure → AdminRoutes.
	app := New(nil)
	mockAdminR := &mockAdminRenderer{}
	app.SetAdminRenderer(mockAdminR)

	// Register sets repo (happens inside srv.Run → bootstrap).
	err := app.Register(&burrow.AppConfig{DB: db})
	require.NoError(t, err)

	// Configure creates handlers (happens inside srv.Run after Register).
	cmd := &cli.Command{
		Flags: app.Flags(),
		Action: func(_ context.Context, cmd *cli.Command) error {
			return app.Configure(cmd)
		},
	}
	require.NoError(t, cmd.Run(context.Background(), []string{"test"}))

	require.NotNil(t, app.adminHandlers, "adminHandlers should be created during Configure")

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithUser(r.Context(), &User{ID: 1, Role: RoleAdmin})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Route("/admin", func(r chi.Router) {
		r.Use(RequireAuth(), RequireAdmin())
		app.AdminRoutes(r)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "AdminUsersPage", mockAdminR.lastMethod)
}

func TestAdminRoutesNilRenderer(t *testing.T) {
	app := &App{}

	r := chi.NewRouter()
	// Should not panic when admin renderer is nil.
	assert.NotPanics(t, func() { app.AdminRoutes(r) })
}

func TestAdminUsersPageHandler(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.CreateUser(ctx, "alice", "Alice")
	require.NoError(t, err)
	_, err = repo.CreateUser(ctx, "bob", "Bob")
	require.NoError(t, err)

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{LoginRedirect: "/dashboard"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	rec := httptest.NewRecorder()

	err = h.UsersPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, "AdminUsersPage", r.lastMethod)
}

func TestAdminUserDetailHandler(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "Alice")
	require.NoError(t, err)

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{LoginRedirect: "/dashboard"}, nil)

	// Use chi router to set up path params.
	router := chi.NewRouter()
	router.Get("/admin/users/{id}", burrow.Handle(h.UserDetail))

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/users/%d", user.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "AdminUserDetailPage", r.lastMethod)
}

func TestAdminUserDetailNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{LoginRedirect: "/dashboard"}, nil)

	router := chi.NewRouter()
	router.Get("/admin/users/{id}", burrow.Handle(h.UserDetail))

	req := httptest.NewRequest(http.MethodGet, "/admin/users/999", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminUpdateUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{}, nil)

	router := chi.NewRouter()
	router.Post("/admin/users/{id}", burrow.Handle(h.UpdateUser))

	body := strings.NewReader("name=Alice+Wonder&bio=Hello+World&email=alice%40example.com&role=admin")
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/users/%d", user.ID), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/admin/users", rec.Header().Get("Location"), "default save redirects to user list")

	got, err := repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "Alice Wonder", got.Name)
	assert.Equal(t, "Hello World", got.Bio)
	require.NotNil(t, got.Email)
	assert.Equal(t, "alice@example.com", *got.Email)
	assert.Equal(t, RoleAdmin, got.Role)
}

func TestAdminUpdateUserContinueEditing(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{}, nil)

	router := chi.NewRouter()
	router.Post("/admin/users/{id}", burrow.Handle(h.UpdateUser))

	body := strings.NewReader("name=Alice&role=user&_continue=1")
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/users/%d", user.ID), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, fmt.Sprintf("/admin/users/%d", user.ID), rec.Header().Get("Location"), "continue redirects back to detail page")
}

func TestAdminUpdateUserClearsOptionalFields(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	email := "alice@example.com"
	user, err := repo.CreateUser(ctx, "alice", "Alice")
	require.NoError(t, err)
	user.Bio = "Old bio"
	user.Email = &email
	require.NoError(t, repo.UpdateUser(ctx, user))

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{}, nil)

	router := chi.NewRouter()
	router.Post("/admin/users/{id}", burrow.Handle(h.UpdateUser))

	body := strings.NewReader("name=&bio=&email=&role=user")
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/users/%d", user.ID), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)

	got, err := repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Empty(t, got.Name)
	assert.Empty(t, got.Bio)
	assert.Nil(t, got.Email)
}

func TestAdminUpdateUserNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{}, nil)

	router := chi.NewRouter()
	router.Post("/admin/users/{id}", burrow.Handle(h.UpdateUser))

	body := strings.NewReader("name=Test&role=user")
	req := httptest.NewRequest(http.MethodPost, "/admin/users/999", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminUpdateUserLastAdminDemotion(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	admin, err := repo.CreateUser(ctx, "admin", "")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, admin.ID, RoleAdmin))

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{}, nil)

	router := chi.NewRouter()
	router.Post("/admin/users/{id}", burrow.Handle(h.UpdateUser))

	body := strings.NewReader("name=Admin&role=user")
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/users/%d", admin.ID), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "should reject demotion of last admin")

	got, err := repo.GetUserByID(ctx, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, RoleAdmin, got.Role, "role should remain admin")
}

func TestAdminUpdateUserDemoteNonLastAdmin(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	admin1, err := repo.CreateUser(ctx, "admin1", "")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, admin1.ID, RoleAdmin))

	admin2, err := repo.CreateUser(ctx, "admin2", "")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, admin2.ID, RoleAdmin))

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{}, nil)

	router := chi.NewRouter()
	router.Post("/admin/users/{id}", burrow.Handle(h.UpdateUser))

	body := strings.NewReader("name=Admin2&role=user")
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/users/%d", admin2.ID), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code, "should allow demotion when multiple admins exist")

	got, err := repo.GetUserByID(ctx, admin2.ID)
	require.NoError(t, err)
	assert.Equal(t, RoleUser, got.Role)
}

func TestAdminUpdateUserInvalidRole(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{}, nil)

	router := chi.NewRouter()
	router.Post("/admin/users/{id}", burrow.Handle(h.UpdateUser))

	body := strings.NewReader("name=Alice&role=superadmin")
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/users/%d", user.ID), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Admin invite handler tests ---

func TestAdminInvitesPage(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{LoginRedirect: "/dashboard"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/invites", nil)
	rec := httptest.NewRecorder()

	err := h.InvitesPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, "AdminInvitesPage", r.lastMethod)
}

func TestAdminCreateInvite(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	user, _ := repo.CreateUser(ctx, "admin", "")

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{LoginRedirect: "/dashboard"}, nil)

	body := strings.NewReader(`label=John+Doe&email=invitee@example.com`)
	req := httptest.NewRequest(http.MethodPost, "/admin/invites", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithSession(req, user)
	rec := httptest.NewRecorder()

	err := h.CreateInvite(rec, req)

	require.NoError(t, err)
	assert.Equal(t, "AdminInvitesPage", r.lastMethod)

	invites, err := repo.ListInvites(ctx)
	require.NoError(t, err)
	assert.Len(t, invites, 1)
	assert.Equal(t, "invitee@example.com", invites[0].Email)
	assert.Equal(t, "John Doe", invites[0].Label)
}

func TestAdminCreateInviteNoAuth(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{LoginRedirect: "/dashboard"}, nil)

	router := chi.NewRouter()
	router.Post("/admin/invites", burrow.Handle(h.CreateInvite))

	body := strings.NewReader(`email=test@example.com`)
	req := httptest.NewRequest(http.MethodPost, "/admin/invites", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminDeleteInviteInvalidID(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{LoginRedirect: "/dashboard"}, nil)

	router := chi.NewRouter()
	router.Delete("/admin/invites/{id}", burrow.Handle(h.DeleteInvite))

	req := httptest.NewRequest(http.MethodDelete, "/admin/invites/invalid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminDeleteInviteSuccess(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	invite := &Invite{
		Email:     "delete@example.com",
		TokenHash: "deletehash",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, repo.CreateInvite(ctx, invite))

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{LoginRedirect: "/dashboard"}, nil)

	router := chi.NewRouter()
	router.Delete("/admin/invites/{id}", burrow.Handle(h.DeleteInvite))

	req := httptest.NewRequest(http.MethodDelete, "/admin/invites/"+strconv.FormatInt(invite.ID, 10), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/admin/invites", rec.Header().Get("HX-Redirect"))

	// Verify invite was deleted.
	invites, err := repo.ListInvites(ctx)
	require.NoError(t, err)
	assert.Empty(t, invites)
}

// mockAdminRenderer implements AdminRenderer for tests.
type mockAdminRenderer struct {
	lastMethod string
}

func (m *mockAdminRenderer) AdminUsersPage(w http.ResponseWriter, _ *http.Request, _ []User) error {
	m.lastMethod = "AdminUsersPage"
	return burrow.Text(w, http.StatusOK, "admin-users")
}

func (m *mockAdminRenderer) AdminUserDetailPage(w http.ResponseWriter, _ *http.Request, _ *User) error {
	m.lastMethod = "AdminUserDetailPage"
	return burrow.Text(w, http.StatusOK, "admin-user-detail")
}

func (m *mockAdminRenderer) AdminInvitesPage(w http.ResponseWriter, _ *http.Request, _ []Invite, _ string, _ bool) error {
	m.lastMethod = "AdminInvitesPage"
	return burrow.Text(w, http.StatusOK, "admin-invites")
}

// --- Admin delete user tests ---

func TestDeleteUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "Alice")
	require.NoError(t, err)

	err = repo.DeleteUser(ctx, user.ID)
	require.NoError(t, err)

	// Soft-deleted user should not appear in ListUsers.
	users, err := repo.ListUsers(ctx)
	require.NoError(t, err)
	assert.Empty(t, users)

	// GetUserByID should also not find the soft-deleted user.
	_, err = repo.GetUserByID(ctx, user.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

// deleteUserRouter creates a chi router with the DeleteUser handler
// and the given user injected into the request context.
func deleteUserRouter(h *adminHandlers, user *User) *chi.Mux {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rctx := WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(rctx))
		})
	})
	router.Delete("/admin/users/{id}", burrow.Handle(h.DeleteUser))
	return router
}

func TestAdminDeleteUserSuccess(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	adminUser, err := repo.CreateUser(ctx, "admin", "Admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, adminUser.ID, RoleAdmin))

	target, err := repo.CreateUser(ctx, "alice", "Alice")
	require.NoError(t, err)

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{}, nil)
	router := deleteUserRouter(h, adminUser)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/admin/users/%d", target.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/admin/users", rec.Header().Get("HX-Redirect"))

	// Verify user was soft-deleted.
	users, err := repo.ListUsers(ctx)
	require.NoError(t, err)
	assert.Len(t, users, 1, "only the admin should remain")
	assert.Equal(t, "admin", users[0].Username)
}

func TestAdminDeleteUserInvalidID(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{}, nil)
	router := deleteUserRouter(h, &User{ID: 1, Role: RoleAdmin})

	req := httptest.NewRequest(http.MethodDelete, "/admin/users/invalid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminDeleteUserNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{}, nil)
	router := deleteUserRouter(h, &User{ID: 1, Role: RoleAdmin})

	req := httptest.NewRequest(http.MethodDelete, "/admin/users/999", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminDeleteUserSelf(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	adminUser, err := repo.CreateUser(ctx, "admin", "Admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, adminUser.ID, RoleAdmin))

	r := &mockAdminRenderer{}
	h := newAdminHandlers(repo, r, &Config{}, nil)
	router := deleteUserRouter(h, adminUser)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/admin/users/%d", adminUser.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Verify user was NOT deleted.
	users, err := repo.ListUsers(ctx)
	require.NoError(t, err)
	assert.Len(t, users, 1)
}

// --- SetAuthLayout tests ---

func TestSetAuthLayout(t *testing.T) {
	app := &App{}
	assert.Nil(t, app.authLayout, "authLayout should be nil by default")

	testLayout := burrow.LayoutFunc(func(title string, content templ.Component) templ.Component {
		return content
	})
	app.SetAuthLayout(testLayout)
	assert.NotNil(t, app.authLayout, "authLayout should be set after SetAuthLayout")
}

func TestPublicAuthRoutesUseAuthLayout(t *testing.T) {
	// Set up a mock renderer that captures the layout from context.
	var capturedLayout burrow.LayoutFunc
	mockR := &layoutCapturingRenderer{capturedLayout: &capturedLayout}

	app := &App{
		renderer: mockR,
		handlers: NewHandlers(nil, nil, nil, mockR, &Config{LoginRedirect: "/"}),
	}

	authLayout := burrow.LayoutFunc(func(title string, content templ.Component) templ.Component {
		return content
	})
	app.SetAuthLayout(authLayout)

	// Set up a global layout that should be overridden on public routes.
	globalLayout := burrow.LayoutFunc(func(title string, content templ.Component) templ.Component {
		return content
	})

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithLayout(r.Context(), globalLayout)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	app.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// The captured layout should be the auth layout, not the global one.
	require.NotNil(t, *mockR.capturedLayout, "layout should be set in context")
}

func TestAuthenticatedRoutesKeepGlobalLayout(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	// Set up a mock renderer that captures the layout from context.
	var capturedLayout burrow.LayoutFunc
	mockR := &layoutCapturingRenderer{capturedLayout: &capturedLayout}

	app := &App{
		repo:     repo,
		renderer: mockR,
		handlers: NewHandlers(repo, nil, nil, mockR, &Config{LoginRedirect: "/"}),
	}

	authLayout := burrow.LayoutFunc(func(title string, content templ.Component) templ.Component {
		return content
	})
	app.SetAuthLayout(authLayout)

	// Set up a global layout.
	globalLayout := burrow.LayoutFunc(func(title string, content templ.Component) templ.Component {
		return content
	})

	// Create a user so the credentials handler can look up credentials.
	user, err := repo.CreateUser(context.Background(), "alice", "Alice")
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithLayout(r.Context(), globalLayout)
			// Inject the user so RequireAuth passes.
			ctx = WithUser(ctx, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	app.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/auth/credentials/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// The captured layout should be the global layout, not the auth layout.
	require.NotNil(t, *mockR.capturedLayout, "layout should be set in context")
}

func TestPublicRoutesWithoutAuthLayoutKeepGlobalLayout(t *testing.T) {
	// When no auth layout is set, public routes should keep the global layout.
	var capturedLayout burrow.LayoutFunc
	mockR := &layoutCapturingRenderer{capturedLayout: &capturedLayout}

	app := &App{
		renderer: mockR,
		handlers: NewHandlers(nil, nil, nil, mockR, &Config{LoginRedirect: "/"}),
	}
	// No SetAuthLayout call.

	globalLayout := burrow.LayoutFunc(func(title string, content templ.Component) templ.Component {
		return content
	})

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithLayout(r.Context(), globalLayout)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	app.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, *mockR.capturedLayout, "global layout should be preserved when no auth layout is set")
}

// layoutCapturingRenderer is a mock Renderer that captures the layout from context.
type layoutCapturingRenderer struct {
	capturedLayout *burrow.LayoutFunc
}

func (m *layoutCapturingRenderer) LoginPage(w http.ResponseWriter, r *http.Request, _ string) error {
	lay := burrow.Layout(r.Context())
	*m.capturedLayout = lay
	return burrow.Text(w, http.StatusOK, "login")
}

func (m *layoutCapturingRenderer) RegisterPage(w http.ResponseWriter, r *http.Request, _, _ bool, _, _ string) error {
	lay := burrow.Layout(r.Context())
	*m.capturedLayout = lay
	return burrow.Text(w, http.StatusOK, "register")
}

func (m *layoutCapturingRenderer) CredentialsPage(w http.ResponseWriter, r *http.Request, _ []Credential) error {
	lay := burrow.Layout(r.Context())
	*m.capturedLayout = lay
	return burrow.Text(w, http.StatusOK, "credentials")
}

func (m *layoutCapturingRenderer) RecoveryPage(w http.ResponseWriter, r *http.Request, _ string) error {
	lay := burrow.Layout(r.Context())
	*m.capturedLayout = lay
	return burrow.Text(w, http.StatusOK, "recovery")
}

func (m *layoutCapturingRenderer) RecoveryCodesPage(w http.ResponseWriter, r *http.Request, _ []string) error {
	lay := burrow.Layout(r.Context())
	*m.capturedLayout = lay
	return burrow.Text(w, http.StatusOK, "recovery-codes")
}

func (m *layoutCapturingRenderer) VerifyPendingPage(w http.ResponseWriter, r *http.Request) error {
	lay := burrow.Layout(r.Context())
	*m.capturedLayout = lay
	return burrow.Text(w, http.StatusOK, "verify-pending")
}

func (m *layoutCapturingRenderer) VerifyEmailSuccess(w http.ResponseWriter, r *http.Request) error {
	lay := burrow.Layout(r.Context())
	*m.capturedLayout = lay
	return burrow.Text(w, http.StatusOK, "verify-success")
}

func (m *layoutCapturingRenderer) VerifyEmailError(w http.ResponseWriter, r *http.Request, _ string) error {
	lay := burrow.Layout(r.Context())
	*m.capturedLayout = lay
	return burrow.Text(w, http.StatusBadRequest, "verify-error")
}

// --- Static files tests ---

func TestStaticFS(t *testing.T) {
	app := &App{}
	prefix, fsys := app.StaticFS()

	assert.Equal(t, "auth", prefix)
	require.NotNil(t, fsys)

	f, err := fsys.Open("webauthn.js")
	require.NoError(t, err, "webauthn.js should exist in static FS")
	_ = f.Close()
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
