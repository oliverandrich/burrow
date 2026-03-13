package auth

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/burrow/contrib/session"
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
	_ burrow.App               = (*App)(nil)
	_ burrow.Migratable        = (*App)(nil)
	_ burrow.Configurable      = (*App)(nil)
	_ burrow.HasMiddleware     = (*App)(nil)
	_ burrow.HasRoutes         = (*App)(nil)
	_ burrow.HasAdmin          = (*App)(nil)
	_ burrow.HasCLICommands    = (*App)(nil)
	_ burrow.HasDependencies   = (*App)(nil)
	_ burrow.HasStaticFiles    = (*App)(nil)
	_ burrow.HasTranslations   = (*App)(nil)
	_ burrow.HasShutdown       = (*App)(nil)
	_ burrow.HasRequestFuncMap = (*App)(nil)
	_ burrow.HasTemplates      = (*App)(nil)
	_ burrow.HasFuncMap        = (*App)(nil)
)

func TestAppName(t *testing.T) {
	app := &App{}
	assert.Equal(t, "auth", app.Name())
}

func TestAppFlags(t *testing.T) {
	app := &App{}
	flags := app.Flags(nil)

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
		"migrations/003_user_is_active.up.sql",
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

func TestValidateRecoveryCodeMatchesLastCode(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	// Create multiple codes — the target is the last one.
	codes := []string{"decoy-aaa", "decoy-bbb", "target-ccc"}
	hashes := make([]string, 0, len(codes))
	for _, code := range codes {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(code), bcrypt.MinCost)
		require.NoError(t, hashErr)
		hashes = append(hashes, string(hash))
	}
	require.NoError(t, repo.CreateRecoveryCodes(ctx, user.ID, hashes))

	// Match the last code.
	valid, err := repo.ValidateAndUseRecoveryCode(ctx, user.ID, "target-ccc")
	require.NoError(t, err)
	assert.True(t, valid)

	// Only one code should be used; two remain.
	count, err := repo.GetUnusedRecoveryCodeCount(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
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

func TestInviteMarkUsedTwiceFails(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	alice, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)
	bob, err := repo.CreateUser(ctx, "bob", "")
	require.NoError(t, err)

	invite := &Invite{
		Email:     "shared@example.com",
		TokenHash: "shared-hash",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	require.NoError(t, repo.CreateInvite(ctx, invite))

	// First use succeeds.
	err = repo.MarkInviteUsed(ctx, invite.ID, alice.ID)
	require.NoError(t, err)

	// Second use fails — invite already consumed.
	err = repo.MarkInviteUsed(ctx, invite.ID, bob.ID)
	assert.ErrorIs(t, err, ErrInviteAlreadyUsed)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
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

// newTestApp creates an App with repo initialized for admin handler tests.
func newTestApp(t *testing.T) (*App, *Repository) {
	t.Helper()
	db := openTestDB(t)
	registry := burrow.NewRegistry()
	registry.Add(session.New())
	app := New()
	registry.Add(app)
	require.NoError(t, registry.RegisterAll(db))
	app.config = &Config{}
	return app, app.repo
}

// stubTemplateExecutor returns a TemplateExecutor that renders template name as plain text.
func stubTemplateExecutor() burrow.TemplateExecutor {
	return func(_ *http.Request, name string, _ map[string]any) (template.HTML, error) {
		return template.HTML("<div>" + name + "</div>"), nil //nolint:gosec // test stub
	}
}

func TestAdminUserDetailHandler(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "Alice")
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rctx := burrow.WithTemplateExecutor(r.Context(), stubTemplateExecutor())
			next.ServeHTTP(w, r.WithContext(rctx))
		})
	})
	router.Get("/admin/users/{id}", burrow.Handle(app.handleUserDetail))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/admin/users/%d", user.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminUserDetailNotFound(t *testing.T) {
	app, _ := newTestApp(t)

	router := chi.NewRouter()
	router.Get("/admin/users/{id}", burrow.Handle(app.handleUserDetail))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/users/999", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminUpdateUser(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Post("/admin/users/{id}", burrow.Handle(app.handleUpdateUser))

	body := strings.NewReader("name=Alice+Wonder&bio=Hello+World&email=alice%40example.com&role=admin")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/admin/users/%d", user.ID), body)
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
	app, repo := newTestApp(t)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Post("/admin/users/{id}", burrow.Handle(app.handleUpdateUser))

	body := strings.NewReader("name=Alice&role=user&_continue=1")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/admin/users/%d", user.ID), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, fmt.Sprintf("/admin/users/%d", user.ID), rec.Header().Get("Location"), "continue redirects back to detail page")
}

func TestAdminUpdateUserClearsOptionalFields(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	email := "alice@example.com"
	user, err := repo.CreateUser(ctx, "alice", "Alice")
	require.NoError(t, err)
	user.Bio = "Old bio"
	user.Email = &email
	require.NoError(t, repo.UpdateUser(ctx, user))

	router := chi.NewRouter()
	router.Post("/admin/users/{id}", burrow.Handle(app.handleUpdateUser))

	body := strings.NewReader("name=&bio=&email=&role=user")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/admin/users/%d", user.ID), body)
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
	app, _ := newTestApp(t)

	router := chi.NewRouter()
	router.Post("/admin/users/{id}", burrow.Handle(app.handleUpdateUser))

	body := strings.NewReader("name=Test&role=user")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/users/999", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminUpdateUserLastAdminDemotion(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	admin, err := repo.CreateUser(ctx, "admin", "")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, admin.ID, RoleAdmin))

	router := chi.NewRouter()
	router.Post("/admin/users/{id}", burrow.Handle(app.handleUpdateUser))

	body := strings.NewReader("name=Admin&role=user")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/admin/users/%d", admin.ID), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "should reject demotion of last admin")

	got, err := repo.GetUserByID(ctx, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, RoleAdmin, got.Role, "role should remain admin")
}

func TestAdminUpdateUserDemoteNonLastAdmin(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	admin1, err := repo.CreateUser(ctx, "admin1", "")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, admin1.ID, RoleAdmin))

	admin2, err := repo.CreateUser(ctx, "admin2", "")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, admin2.ID, RoleAdmin))

	router := chi.NewRouter()
	router.Post("/admin/users/{id}", burrow.Handle(app.handleUpdateUser))

	body := strings.NewReader("name=Admin2&role=user")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/admin/users/%d", admin2.ID), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code, "should allow demotion when multiple admins exist")

	got, err := repo.GetUserByID(ctx, admin2.ID)
	require.NoError(t, err)
	assert.Equal(t, RoleUser, got.Role)
}

func TestAdminUpdateUserInvalidRole(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "")
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Post("/admin/users/{id}", burrow.Handle(app.handleUpdateUser))

	body := strings.NewReader("name=Alice&role=superadmin")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/admin/users/%d", user.ID), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Admin invite handler tests ---

func TestAdminCreateInvite(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()
	user, _ := repo.CreateUser(ctx, "admin", "")

	body := strings.NewReader(`label=John+Doe&email=invitee@example.com`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/invites", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithSession(req, user)
	rec := httptest.NewRecorder()

	err := app.handleCreateInvite(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/admin/invites", rec.Header().Get("Location"))

	invites, err := repo.ListInvites(ctx)
	require.NoError(t, err)
	assert.Len(t, invites, 1)
	assert.Equal(t, "invitee@example.com", invites[0].Email)
	assert.Equal(t, "John Doe", invites[0].Label)

	// Verify flash message contains the prefix and the registration URL.
	sessionValues := session.GetValues(req)
	require.NotNil(t, sessionValues)
	storedMsgs, ok := sessionValues["_messages"].([]messages.Message)
	require.True(t, ok, "expected messages in session")
	require.Len(t, storedMsgs, 1)
	assert.Contains(t, storedMsgs[0].Text, "admin-invites-copy-url")
	assert.Contains(t, storedMsgs[0].Text, "/auth/register?invite=")
	assert.Equal(t, messages.Success, storedMsgs[0].Level)
}

func TestAdminCreateInviteNoAuth(t *testing.T) {
	app, _ := newTestApp(t)

	router := chi.NewRouter()
	router.Post("/admin/invites", burrow.Handle(app.handleCreateInvite))

	body := strings.NewReader(`email=test@example.com`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/invites", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRevokeInviteInvalidID(t *testing.T) {
	_, repo := newTestApp(t)

	router := chi.NewRouter()
	router.Delete("/admin/invites/{id}/revoke", burrow.Handle(revokeInviteHandler(repo)))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/admin/invites/invalid/revoke", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRevokeInviteSuccess(t *testing.T) {
	_, repo := newTestApp(t)
	ctx := context.Background()

	invite := &Invite{
		Email:     "delete@example.com",
		TokenHash: "deletehash",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, repo.CreateInvite(ctx, invite))

	router := chi.NewRouter()
	router.Delete("/admin/invites/{id}/revoke", burrow.Handle(revokeInviteHandler(repo)))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/admin/invites/"+strconv.FormatInt(invite.ID, 10)+"/revoke", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/admin/invites", rec.Header().Get("HX-Redirect"))

	// Verify invite was deleted.
	invites, err := repo.ListInvites(ctx)
	require.NoError(t, err)
	assert.Empty(t, invites)
}

func TestIsRevokable(t *testing.T) {
	active := Invite{ExpiresAt: time.Now().Add(time.Hour)}
	assert.True(t, isRevokable(active))

	expired := Invite{ExpiresAt: time.Now().Add(-time.Hour)}
	assert.False(t, isRevokable(expired))

	now := time.Now()
	used := Invite{ExpiresAt: time.Now().Add(time.Hour), UsedAt: &now}
	assert.False(t, isRevokable(used))

	assert.False(t, isRevokable("not an invite"))
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

// deleteUserRouter creates a chi router with the handleDeleteUser handler
// and the given user injected into the request context.
func deleteUserRouter(app *App, user *User) *chi.Mux {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rctx := WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(rctx))
		})
	})
	router.Delete("/admin/users/{id}", burrow.Handle(app.handleDeleteUser))
	return router
}

func TestAdminDeleteUserSuccess(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	adminUser, err := repo.CreateUser(ctx, "admin", "Admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, adminUser.ID, RoleAdmin))

	target, err := repo.CreateUser(ctx, "alice", "Alice")
	require.NoError(t, err)

	router := deleteUserRouter(app, adminUser)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, fmt.Sprintf("/admin/users/%d", target.ID), nil)
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
	app, _ := newTestApp(t)
	router := deleteUserRouter(app, &User{ID: 1, Role: RoleAdmin})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/admin/users/invalid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminDeleteUserNotFound(t *testing.T) {
	app, _ := newTestApp(t)
	router := deleteUserRouter(app, &User{ID: 1, Role: RoleAdmin})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/admin/users/999", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminDeleteUserSelf(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	adminUser, err := repo.CreateUser(ctx, "admin", "Admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, adminUser.ID, RoleAdmin))

	router := deleteUserRouter(app, adminUser)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, fmt.Sprintf("/admin/users/%d", adminUser.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Verify user was NOT deleted.
	users, err := repo.ListUsers(ctx)
	require.NoError(t, err)
	assert.Len(t, users, 1)
}

func TestAdminDeleteUserLastAdmin(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	// Create the only admin and a regular user.
	onlyAdmin, err := repo.CreateUser(ctx, "onlyadmin", "Only Admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, onlyAdmin.ID, RoleAdmin))

	otherUser, err := repo.CreateUser(ctx, "other", "Other")
	require.NoError(t, err)

	// Another user tries to delete the last admin — must be rejected.
	router := deleteUserRouter(app, otherUser)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, fmt.Sprintf("/admin/users/%d", onlyAdmin.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "deleting the last admin should be rejected")

	// Verify the admin was NOT deleted.
	users, err := repo.ListUsers(ctx)
	require.NoError(t, err)
	assert.Len(t, users, 2, "both users should still exist")
}

// --- PurgeOrphanedUsers tests ---

func TestPurgeOrphanedUsersDeletesUsersWithoutCredentials(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create a user with no credentials (simulates abandoned registration).
	orphan, err := repo.CreateUser(ctx, "orphan", "")
	require.NoError(t, err)

	// Backdate the user's created_at so it qualifies for cleanup.
	_, err = db.NewUpdate().Model((*User)(nil)).
		Set("created_at = datetime('now', '-10 minutes')").
		Where("id = ?", orphan.ID).
		Exec(ctx)
	require.NoError(t, err)

	// Create a user WITH credentials (should be kept).
	legit, err := repo.CreateUser(ctx, "legit", "")
	require.NoError(t, err)
	_, err = db.NewUpdate().Model((*User)(nil)).
		Set("created_at = datetime('now', '-10 minutes')").
		Where("id = ?", legit.ID).
		Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.CreateCredential(ctx, &Credential{
		UserID:       legit.ID,
		CredentialID: []byte("cred1"),
		PublicKey:    []byte("key1"),
		Name:         "Passkey",
	}))

	// Purge users with 0 credentials older than 5 minutes.
	purged, err := repo.PurgeOrphanedUsers(ctx, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 1, purged)

	// Orphan should be gone (hard-deleted).
	_, err = repo.GetUserByID(ctx, orphan.ID)
	require.ErrorIs(t, err, ErrNotFound)

	// Legit user should still exist.
	_, err = repo.GetUserByID(ctx, legit.ID)
	require.NoError(t, err)
}

func TestPurgeOrphanedUsersSkipsRecentUsers(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create a user with no credentials (just created — not yet orphaned).
	_, err := repo.CreateUser(ctx, "newuser", "")
	require.NoError(t, err)

	// Purge with 5-minute threshold — should not delete the recent user.
	purged, err := repo.PurgeOrphanedUsers(ctx, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 0, purged)
}

// --- SetUserActive / Deactivate / Activate tests ---

func TestSetUserActive(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice", "Alice")
	require.NoError(t, err)
	assert.True(t, user.IsActive, "new users should be active by default")

	// Deactivate.
	require.NoError(t, repo.SetUserActive(ctx, user.ID, false))
	updated, err := repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.False(t, updated.IsActive)

	// Reactivate.
	require.NoError(t, repo.SetUserActive(ctx, user.ID, true))
	updated, err = repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, updated.IsActive)
}

func TestIsDeactivatable(t *testing.T) {
	assert.True(t, isDeactivatable(User{IsActive: true}))
	assert.False(t, isDeactivatable(User{IsActive: false}))
	assert.False(t, isDeactivatable("not a user"))
}

func TestIsActivatable(t *testing.T) {
	assert.True(t, isActivatable(User{IsActive: false}))
	assert.False(t, isActivatable(User{IsActive: true}))
	assert.False(t, isActivatable("not a user"))
}

// userActionRouter creates a chi router with a POST handler and user context.
func userActionRouter(handler burrow.HandlerFunc, user *User) *chi.Mux {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rctx := WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(rctx))
		})
	})
	router.Post("/admin/users/{id}/deactivate", burrow.Handle(handler))
	router.Post("/admin/users/{id}/activate", burrow.Handle(handler))
	return router
}

func TestDeactivateUserSuccess(t *testing.T) {
	_, repo := newTestApp(t)
	ctx := context.Background()

	adminUser, err := repo.CreateUser(ctx, "admin", "Admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, adminUser.ID, RoleAdmin))

	target, err := repo.CreateUser(ctx, "alice", "Alice")
	require.NoError(t, err)

	router := userActionRouter(deactivateUserHandler(repo), adminUser)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/admin/users/%d/deactivate", target.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/admin/users", rec.Header().Get("HX-Redirect"))

	updated, err := repo.GetUserByID(ctx, target.ID)
	require.NoError(t, err)
	assert.False(t, updated.IsActive)
}

func TestDeactivateUserSelf(t *testing.T) {
	_, repo := newTestApp(t)
	ctx := context.Background()

	adminUser, err := repo.CreateUser(ctx, "admin", "Admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, adminUser.ID, RoleAdmin))

	router := userActionRouter(deactivateUserHandler(repo), adminUser)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/admin/users/%d/deactivate", adminUser.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// User should still be active.
	updated, err := repo.GetUserByID(ctx, adminUser.ID)
	require.NoError(t, err)
	assert.True(t, updated.IsActive)
}

func TestActivateUserSuccess(t *testing.T) {
	_, repo := newTestApp(t)
	ctx := context.Background()

	adminUser, err := repo.CreateUser(ctx, "admin", "Admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, adminUser.ID, RoleAdmin))

	target, err := repo.CreateUser(ctx, "alice", "Alice")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserActive(ctx, target.ID, false))

	router := userActionRouter(activateUserHandler(repo), adminUser)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/admin/users/%d/activate", target.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/admin/users", rec.Header().Get("HX-Redirect"))

	updated, err := repo.GetUserByID(ctx, target.ID)
	require.NoError(t, err)
	assert.True(t, updated.IsActive)
}

// --- WithAuthLayout option tests ---

func TestWithAuthLayoutOption(t *testing.T) {
	app := New(WithAuthLayout("test/layout"))
	assert.Equal(t, "test/layout", app.authLayout, "authLayout should be set via WithAuthLayout option")
}

func TestPublicAuthRoutesUseAuthLayout(t *testing.T) {
	// Set up a mock renderer that captures the layout from context.
	var capturedLayout string
	mockR := &layoutCapturingRenderer{capturedLayout: &capturedLayout}

	app := &App{
		renderer: mockR,
		handlers: NewHandlers(nil, nil, nil, mockR, &Config{LoginRedirect: "/"}, &App{withLocale: testI18nBundle(t).WithLocale}),
	}

	app.authLayout = "auth/layout"

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithLayout(r.Context(), "global/layout")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	app.Routes(r)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// The captured layout should be the auth layout, not the global one.
	assert.Equal(t, "auth/layout", *mockR.capturedLayout, "layout should be the auth layout")
}

func TestAuthenticatedRoutesKeepGlobalLayout(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	// Set up a mock renderer that captures the layout from context.
	var capturedLayout string
	mockR := &layoutCapturingRenderer{capturedLayout: &capturedLayout}

	app := &App{
		repo:     repo,
		renderer: mockR,
		handlers: NewHandlers(repo, nil, nil, mockR, &Config{LoginRedirect: "/"}, &App{withLocale: testI18nBundle(t).WithLocale}),
	}

	app.authLayout = "auth/layout"

	// Create a user so the credentials handler can look up credentials.
	user, err := repo.CreateUser(context.Background(), "alice", "Alice")
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithLayout(r.Context(), "global/layout")
			// Inject the user so RequireAuth passes.
			ctx = WithUser(ctx, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	app.Routes(r)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/credentials/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// The captured layout should be the global layout, not the auth layout.
	assert.Equal(t, "global/layout", *mockR.capturedLayout, "layout should be the global layout")
}

func TestPublicRoutesWithoutAuthLayoutKeepGlobalLayout(t *testing.T) {
	// When no auth layout is set, public routes should keep the global layout.
	var capturedLayout string
	mockR := &layoutCapturingRenderer{capturedLayout: &capturedLayout}

	app := &App{
		renderer: mockR,
		handlers: NewHandlers(nil, nil, nil, mockR, &Config{LoginRedirect: "/"}, &App{withLocale: testI18nBundle(t).WithLocale}),
	}
	// No SetAuthLayout call.

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithLayout(r.Context(), "global/layout")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	app.Routes(r)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "global/layout", *mockR.capturedLayout, "global layout should be preserved when no auth layout is set")
}

// layoutCapturingRenderer is a mock Renderer that captures the layout from context.
type layoutCapturingRenderer struct {
	capturedLayout *string
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

func TestRequestFuncMap(t *testing.T) {
	app := &App{}
	user := &User{ID: 1, Username: "alice"}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	ctx := WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	fm := app.RequestFuncMap(req)

	currentUserFunc := fm["currentUser"].(func() *User)
	assert.Equal(t, user, currentUserFunc())

	isAuthFunc := fm["isAuthenticated"].(func() bool)
	assert.True(t, isAuthFunc())
}

func TestRequestFuncMapUnauthenticated(t *testing.T) {
	app := &App{}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

	fm := app.RequestFuncMap(req)

	currentUserFunc := fm["currentUser"].(func() *User)
	assert.Nil(t, currentUserFunc())

	isAuthFunc := fm["isAuthenticated"].(func() bool)
	assert.False(t, isAuthFunc())
}

// --- Option functions ---

func TestWithRendererOption(t *testing.T) {
	r := &mockRenderer{}
	app := New(WithRenderer(r))
	assert.Equal(t, r, app.renderer)
}

func TestWithLogoComponentOption(t *testing.T) {
	logo := template.HTML(`<img src="logo.png"/>`)
	app := New(WithLogoComponent(logo))
	assert.Equal(t, logo, app.logo)
}

func TestWithEmailServiceOption(t *testing.T) {
	svc := &mockEmailService{}
	app := New(WithEmailService(svc))
	assert.Equal(t, svc, app.emailService)
}

func TestTranslationFS(t *testing.T) {
	app := &App{}
	fsys := app.TranslationFS()
	require.NotNil(t, fsys)
}

func TestTemplateFS(t *testing.T) {
	app := &App{}
	fsys := app.TemplateFS()
	require.NotNil(t, fsys)
}

func TestFuncMap(t *testing.T) {
	app := &App{}
	fm := app.FuncMap()
	require.NotNil(t, fm)

	// Test credName function.
	credNameFunc, ok := fm["credName"].(func(Credential) string)
	require.True(t, ok)
	assert.Equal(t, "My Key", credNameFunc(Credential{Name: "My Key"}))
	assert.Equal(t, "Passkey", credNameFunc(Credential{Name: ""}))

	// Test emailValue function.
	emailValueFunc, ok := fm["emailValue"].(func(*User) string)
	require.True(t, ok)
	email := "alice@example.com"
	assert.Equal(t, "alice@example.com", emailValueFunc(&User{Email: &email}))
	assert.Empty(t, emailValueFunc(&User{}))

	// Test deref function.
	derefFunc, ok := fm["deref"].(func(*string) string)
	require.True(t, ok)
	s := "hello"
	assert.Equal(t, "hello", derefFunc(&s))
	assert.Empty(t, derefFunc(nil))
}

func TestShutdown(t *testing.T) {
	// Shutdown with nil cancelCleanup should not panic.
	app := &App{}
	err := app.Shutdown(context.Background())
	require.NoError(t, err)

	// Shutdown with a real cancel function.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app2 := &App{cancelCleanup: cancel}
	err = app2.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestShutdownMultipleCalls(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	app := &App{cancelCleanup: cancel}

	// First call should be fine.
	assert.NoError(t, app.Shutdown(context.Background()))
	// Second call should also be safe (cancel is idempotent).
	assert.NoError(t, app.Shutdown(context.Background()))
}

func TestRepoAccessor(t *testing.T) {
	repo := &Repository{}
	app := &App{repo: repo}
	assert.Same(t, repo, app.Repo())
}

func TestHandlersAccessor(t *testing.T) {
	handlers := &Handlers{}
	app := &App{handlers: handlers}
	assert.Same(t, handlers, app.Handlers())
}

func TestAuthMiddlewareWithValidUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	user, err := repo.CreateUser(context.Background(), "alice", "Alice")
	require.NoError(t, err)

	app := &App{repo: repo, config: &Config{LoginRedirect: "/dashboard"}}

	var gotUser *User
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req = session.Inject(req, map[string]any{"user_id": user.ID})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, gotUser)
	assert.Equal(t, "alice", gotUser.Username)
}

func TestAuthMiddlewareSetsAuthChecker(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	user, err := repo.CreateUser(context.Background(), "bob", "Bob")
	require.NoError(t, err)

	app := &App{repo: repo, config: &Config{LoginRedirect: "/dashboard"}}

	t.Run("regular user sets authenticated checker", func(t *testing.T) {
		var gotChecker burrow.AuthChecker
		var hasChecker bool
		handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotChecker, hasChecker = burrow.ContextValue[burrow.AuthChecker](r.Context(), burrow.AuthCheckerContextKey())
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
		req = session.Inject(req, map[string]any{"user_id": user.ID})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.True(t, hasChecker, "AuthChecker should be set in context")
		assert.True(t, gotChecker.IsAuthenticated(), "should report authenticated")
		assert.False(t, gotChecker.IsAdmin(), "regular user should not be admin")
	})

	t.Run("admin user sets admin checker", func(t *testing.T) {
		require.NoError(t, repo.SetUserRole(context.Background(), user.ID, RoleAdmin))

		var gotChecker burrow.AuthChecker
		var hasChecker bool
		handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotChecker, hasChecker = burrow.ContextValue[burrow.AuthChecker](r.Context(), burrow.AuthCheckerContextKey())
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
		req = session.Inject(req, map[string]any{"user_id": user.ID})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.True(t, hasChecker, "AuthChecker should be set in context")
		assert.True(t, gotChecker.IsAuthenticated(), "should report authenticated")
		assert.True(t, gotChecker.IsAdmin(), "admin user should report admin")
	})
}

func TestAuthMiddlewareNoAuthCheckerWhenUnauthenticated(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	app := &App{repo: repo, config: &Config{LoginRedirect: "/dashboard"}}

	var hasChecker bool
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasChecker = burrow.ContextValue[burrow.AuthChecker](r.Context(), burrow.AuthCheckerContextKey())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.False(t, hasChecker, "no AuthChecker should be set for unauthenticated requests")
}

func TestAuthMiddlewareWithInactiveUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	user, err := repo.CreateUser(context.Background(), "inactive", "")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserActive(context.Background(), user.ID, false))

	app := &App{repo: repo, config: &Config{LoginRedirect: "/dashboard"}}

	var gotUser *User
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req = session.Inject(req, map[string]any{"user_id": user.ID})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Nil(t, gotUser, "inactive user should not be set in context")
}

func TestAuthMiddlewareWithNonexistentUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	app := &App{repo: repo, config: &Config{LoginRedirect: "/dashboard"}}

	var gotUser *User
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req = session.Inject(req, map[string]any{"user_id": int64(999)})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Nil(t, gotUser, "nonexistent user should not be set in context")
}

func TestAuthLogoMiddleware(t *testing.T) {
	logo := template.HTML(`<img src="logo.png"/>`)
	mw := authLogoMiddleware(logo)

	var gotLogo template.HTML
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLogo = LogoFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, logo, gotLogo)
}

func TestCredNameWithName(t *testing.T) {
	assert.Equal(t, "My Key", credName(Credential{Name: "My Key"}))
}

func TestCredNameWithoutName(t *testing.T) {
	assert.Equal(t, "Passkey", credName(Credential{Name: ""}))
}

func TestRequestFuncMapAdminEditFlags(t *testing.T) {
	app := &App{}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	ctx := withAdminEditFlags(req.Context(), true, true)
	req = req.WithContext(ctx)

	fm := app.RequestFuncMap(req)

	isSelfFunc := fm["isAdminEditSelf"].(func() bool)
	assert.True(t, isSelfFunc())

	isLastAdminFunc := fm["isAdminEditLastAdmin"].(func() bool)
	assert.True(t, isLastAdminFunc())
}

func TestRequestFuncMapAuthLogo(t *testing.T) {
	app := &App{}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	logo := template.HTML(`<span>Logo</span>`)
	ctx := WithLogo(req.Context(), logo)
	req = req.WithContext(ctx)

	fm := app.RequestFuncMap(req)
	logoFunc := fm["authLogo"].(func() template.HTML)
	assert.Equal(t, logo, logoFunc())
}

func TestNewWithMultipleOptions(t *testing.T) {
	r := &mockRenderer{}
	logo := template.HTML(`<span>Logo</span>`)
	emailSvc := &mockEmailService{}

	app := New(
		WithRenderer(r),
		WithLogoComponent(logo),
		WithEmailService(emailSvc),
		WithAuthLayout("custom/auth-layout"),
	)

	assert.Equal(t, r, app.renderer)
	assert.Equal(t, logo, app.logo)
	assert.Equal(t, emailSvc, app.emailService)
	assert.Equal(t, "custom/auth-layout", app.authLayout)
}

func TestDependencies(t *testing.T) {
	app := &App{}
	deps := app.Dependencies()
	require.Len(t, deps, 1)
	assert.Equal(t, "session", deps[0])
}

func TestAdminRoutes(t *testing.T) {
	app, _ := newTestApp(t)

	// AdminRoutes should not panic when called.
	router := chi.NewRouter()
	app.AdminRoutes(router)
}

func TestRoutesWithLogoMiddleware(t *testing.T) {
	app := &App{
		renderer: &mockRenderer{},
		logo:     template.HTML(`<img src="logo.png"/>`),
	}
	app.handlers = NewHandlers(nil, nil, nil, app.renderer, &Config{LoginRedirect: "/"}, &App{withLocale: testI18nBundle(t).WithLocale})

	router := chi.NewRouter()
	// Should not panic.
	app.Routes(router)

	// Verify logo middleware works by hitting the login page.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestConfigure(t *testing.T) {
	db := openTestDB(t)
	registry := burrow.NewRegistry()
	registry.Add(session.New())
	app := New()
	registry.Add(app)
	require.NoError(t, registry.RegisterAll(db))

	// Build a CLI command that sets the flags and calls Configure.
	cliCmd := &cli.Command{
		Name:  "test",
		Flags: app.Flags(nil),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return app.Configure(cmd)
		},
	}

	err := cliCmd.Run(context.Background(), []string{
		"test",
		"--webauthn-rp-id", "localhost",
		"--webauthn-rp-display-name", "Test App",
		"--webauthn-rp-origin", "http://localhost:8080",
		"--auth-login-redirect", "/home",
		"--auth-logout-redirect", "/goodbye",
	})
	require.NoError(t, err)

	// Verify configuration was applied.
	require.NotNil(t, app.config)
	assert.Equal(t, "/home", app.config.LoginRedirect)
	assert.Equal(t, "/goodbye", app.config.LogoutRedirect)
	require.NotNil(t, app.handlers)
	require.NotNil(t, app.cancelCleanup)

	// Clean up.
	require.NoError(t, app.Shutdown(context.Background()))
}

func TestConfigureWithDefaultOrigin(t *testing.T) {
	db := openTestDB(t)
	registry := burrow.NewRegistry()
	registry.Add(session.New())
	app := New()
	registry.Add(app)
	require.NoError(t, registry.RegisterAll(db))
	app.globalConfig = &burrow.Config{}

	cliCmd := &cli.Command{
		Name:  "test",
		Flags: app.Flags(nil),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return app.Configure(cmd)
		},
	}

	// No --webauthn-rp-origin set, should fallback to base URL.
	err := cliCmd.Run(context.Background(), []string{
		"test",
		"--webauthn-rp-id", "localhost",
		"--webauthn-rp-display-name", "Test App",
	})
	require.NoError(t, err)
	require.NotNil(t, app.config)

	require.NoError(t, app.Shutdown(context.Background()))
}

func TestCleanupOrphanedUsersStopsOnCancel(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	app := &App{repo: repo}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		app.cleanupOrphanedUsers(ctx)
		close(done)
	}()

	// Cancel immediately to test the context cancellation path.
	cancel()

	select {
	case <-done:
		// Goroutine exited cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("cleanupOrphanedUsers did not stop within 2 seconds")
	}
}

func TestCLIPromoteNoArgs(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
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
	err := cliCmd.Run(context.Background(), []string{"test", "promote"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "username is required")
}

func TestCLIPromoteUserNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
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
	err := cliCmd.Run(context.Background(), []string{"test", "promote", "nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCLICreateInviteNoArgs(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	app := &App{repo: repo}
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
	err := cliCmd.Run(context.Background(), []string{"test", "create-invite"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email is required")
}

func TestAdminCreateInviteWithEmailMode(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()
	emailSvc := &mockEmailService{}
	app.emailService = emailSvc
	app.config = &Config{UseEmail: true, BaseURL: "http://localhost:8080"}

	user, _ := repo.CreateUser(ctx, "admin", "")

	body := strings.NewReader(`label=Test&email=invitee@example.com`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/invites", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithSession(req, user)
	rec := httptest.NewRecorder()

	err := app.handleCreateInvite(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.True(t, emailSvc.sendCalled)
}

func TestAdminCreateInviteEmailModeMissingEmail(t *testing.T) {
	app, _ := newTestApp(t)
	app.config = &Config{UseEmail: true}

	body := strings.NewReader(`label=Test`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/invites", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithSession(req, &User{ID: 1})
	rec := httptest.NewRecorder()

	err := app.handleCreateInvite(rec, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email is required")
}
