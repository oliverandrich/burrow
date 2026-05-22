package auth

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/burrowtest"
	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/oliverandrich/burrow/forms"
	"github.com/oliverandrich/burrow/i18n"
	"github.com/oliverandrich/burrow/registry"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"golang.org/x/crypto/bcrypt"
)

// Compile-time interface assertions.
var (
	_ burrow.App               = (*App[EmptyProfile])(nil)
	_ burrow.HasDocuments      = (*App[EmptyProfile])(nil)
	_ burrow.Configurable      = (*App[EmptyProfile])(nil)
	_ burrow.HasMiddleware     = (*App[EmptyProfile])(nil)
	_ burrow.HasRoutes         = (*App[EmptyProfile])(nil)
	_ burrow.HasAdmin          = (*App[EmptyProfile])(nil)
	_ burrow.HasCLICommands    = (*App[EmptyProfile])(nil)
	_ burrow.HasDependencies   = (*App[EmptyProfile])(nil)
	_ burrow.HasStaticFiles    = (*App[EmptyProfile])(nil)
	_ burrow.HasTranslations   = (*App[EmptyProfile])(nil)
	_ burrow.HasShutdown       = (*App[EmptyProfile])(nil)
	_ burrow.Startable         = (*App[EmptyProfile])(nil)
	_ burrow.HasRequestFuncMap = (*App[EmptyProfile])(nil)
	_ burrow.HasTemplates      = (*App[EmptyProfile])(nil)
	_ burrow.HasFuncMap        = (*App[EmptyProfile])(nil)
	_ burrow.HasFlags          = (*App[EmptyProfile])(nil)
)

func TestAppName(t *testing.T) {
	app := &App[EmptyProfile]{}
	assert.Equal(t, "auth", app.Name())
}

func TestAppFlags(t *testing.T) {
	app := &App[EmptyProfile]{}
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
	assert.True(t, names["auth-webauthn-rp-id"])
	assert.True(t, names["auth-webauthn-rp-display-name"])
	assert.True(t, names["auth-webauthn-rp-origin"])
}

// --- WebAuthn RP-ID derivation tests ---

func TestResolveRPID_ExplicitFlagWins(t *testing.T) {
	assert.Equal(t, "example.com", resolveRPID("example.com", "https://app.example.com"))
}

func TestResolveRPID_FallsBackToBaseURLHost(t *testing.T) {
	assert.Equal(t, "app.example.com", resolveRPID("", "https://app.example.com"))
}

func TestResolveRPID_StripsPortFromBaseURL(t *testing.T) {
	assert.Equal(t, "localhost", resolveRPID("", "http://localhost:8080"))
}

func TestResolveRPID_EmptyWhenBothMissing(t *testing.T) {
	assert.Empty(t, resolveRPID("", ""))
}

func TestResolveRPID_EmptyOnMalformedBaseURL(t *testing.T) {
	assert.Empty(t, resolveRPID("", "://not a url"))
}

func TestDocuments(t *testing.T) {
	app := &App[EmptyProfile]{}
	docs := app.Documents()
	require.NotEmpty(t, docs)
	assert.GreaterOrEqual(t, len(docs), 3, "should have at least User, Credential, and RecoveryCode")
}

// --- Test helpers ---

func openTestDB(t *testing.T) *den.DB {
	t.Helper()
	db := burrowtest.DB(t)

	app := New[EmptyProfile]()
	err := den.Register(t.Context(), db, app.Documents()...)
	require.NoError(t, err)

	return db
}

// --- User tests ---

func TestCreateAndGetUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)
	require.NotEmpty(t, user.ID)
	assert.Equal(t, "alice", user.Username)

	got, err := repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "alice", got.Username)
}

func TestCreateUserWithEmail(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUserWithEmail(ctx, "alice@example.com")
	require.NoError(t, err)
	require.NotNil(t, user.Email)
	assert.Equal(t, "alice@example.com", *user.Email)

	got, err := repo.GetUserByEmail(ctx, "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
}

func TestGetUserByUsername(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	_, err := repo.CreateUser(ctx, "bob")
	require.NoError(t, err)

	got, err := repo.GetUserByUsername(ctx, "bob")
	require.NoError(t, err)
	assert.Equal(t, "bob", got.Username)
}

func TestGetUserNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	_, err := repo.GetUserByID(ctx, "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUserExistsAndCount(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	exists, err := repo.UserExists(ctx, "alice")
	require.NoError(t, err)
	assert.False(t, exists)

	_, err = repo.CreateUser(ctx, "alice")
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
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	count, err := repo.CountAdminUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	alice, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, alice.ID, RoleAdmin))

	count, err = repo.CountAdminUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	bob, err := repo.CreateUser(ctx, "bob")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, bob.ID, RoleAdmin))

	count, err = repo.CountAdminUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestSetUserRole(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
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
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	t.Run("empty database", func(t *testing.T) {
		users, err := repo.ListUsers(ctx)
		require.NoError(t, err)
		assert.Empty(t, users)
	})

	t.Run("returns all users ordered by id asc", func(t *testing.T) {
		_, err := repo.CreateUser(ctx, "alice")
		require.NoError(t, err)
		_, err = repo.CreateUser(ctx, "bob")
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
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUserWithEmail(ctx, "alice@example.com")
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
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
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
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
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
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
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
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
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
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
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
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
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

func TestDeleteUserEmailVerificationTokensViaApp(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
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
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	invite := &Invite{
		Email:     "bob@example.com",
		TokenHash: "invite-hash-1",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	err := repo.CreateInvite(ctx, invite)
	require.NoError(t, err)
	require.NotEmpty(t, invite.ID)

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
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
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
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	alice, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)
	bob, err := repo.CreateUser(ctx, "bob")
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
	repo := NewRepository[EmptyProfile](db)
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
	app := &App[EmptyProfile]{config: &Config{LoginRedirect: "/dashboard"}}

	r := chi.NewRouter()
	for _, mw := range app.Middleware() {
		r.Use(mw)
	}

	var gotUser *User[EmptyProfile]
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		gotUser = CurrentUser[EmptyProfile](r.Context())
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
	app := &App[EmptyProfile]{}
	cmds := app.CLICommands()

	require.NotEmpty(t, cmds)

	names := make(map[string]bool)
	for _, cmd := range cmds {
		names[cmd.Name] = true
	}

	assert.True(t, names["set-role"], "should have set-role command")
	assert.True(t, names["create-invite"], "should have create-invite command")
}

func findSetRoleCmd(t *testing.T, app *App[EmptyProfile]) *cli.Command {
	t.Helper()
	for _, cmd := range app.CLICommands() {
		if cmd.Name == "set-role" {
			return cmd
		}
	}
	t.Fatal("set-role command not found")
	return nil
}

func TestCLISetRoleTransitions(t *testing.T) {
	cases := []struct {
		name      string
		fromRole  string
		targetArg string
		wantRole  string
	}{
		{"user → admin", RoleUser, "admin", RoleAdmin},
		{"admin → user", RoleAdmin, "user", RoleUser},
		{"user → staff", RoleUser, "staff", RoleStaff},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t)
			repo := NewRepository[EmptyProfile](db)
			ctx := context.Background()

			user, err := repo.CreateUser(ctx, "subject")
			require.NoError(t, err)
			if tc.fromRole != user.Role {
				require.NoError(t, repo.SetUserRole(ctx, user.ID, tc.fromRole))
			}

			app := &App[EmptyProfile]{repo: repo}
			setRoleCmd := findSetRoleCmd(t, app)
			cliCmd := &cli.Command{
				Name:     "test",
				Action:   func(ctx context.Context, cmd *cli.Command) error { return nil },
				Commands: []*cli.Command{setRoleCmd},
			}

			err = cliCmd.Run(ctx, []string{"test", "set-role", "subject", tc.targetArg})
			require.NoError(t, err)

			got, err := repo.GetUserByID(ctx, user.ID)
			require.NoError(t, err)
			assert.Equal(t, tc.wantRole, got.Role)
		})
	}
}

// TestCLISetRoleStaffPredicates pins the IsStaff/IsAdmin invariants for the
// new tier — covered separately from the transition table so the assertion
// names show up clearly.
func TestCLISetRoleStaffPredicates(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "carol")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, user.ID, RoleStaff))

	got, err := repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, got.IsStaff(), "staff role must report IsStaff() true")
	assert.False(t, got.IsAdmin(), "staff role must not report IsAdmin() true")
}

func TestCLICreateInvite(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	app := &App[EmptyProfile]{repo: repo, globalConfig: &burrow.Config{}}
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

// TestCLISetRoleThroughServerCLICommands is the end-to-end variant of
// TestCLISetRolePromote: it goes through the framework's boot lifecycle
// (Server.CLICommands -> wrapped Action -> Server.boot -> auth.Configure ->
// set-role.Action) rather than constructing &App[EmptyProfile]{repo: repo}
// directly. Without the wrap, the subcommand fires before Configure() runs,
// a.repo is nil, and the action fails with "auth app not initialized".
func TestCLISetRoleThroughServerCLICommands(t *testing.T) {
	dsn := burrowtest.TempDSN(t)
	ctx := t.Context()

	// Pre-seed: open the DB ourselves and create alice, then close. The CLI
	// subcommand opens its own connection after; SQLite's WAL backend handles
	// serial reopens cleanly. We do the same dance again at the end to verify.
	{
		db, err := burrow.OpenDB(ctx, dsn)
		require.NoError(t, err)
		app := New[EmptyProfile]()
		require.NoError(t, den.Register(ctx, db, app.Documents()...))
		repo := NewRepository[EmptyProfile](db)
		_, err = repo.CreateUser(ctx, "alice")
		require.NoError(t, err)
		require.NoError(t, db.Close())
	}

	// Wire up Server with auth registered (plus auth's declared dependencies)
	// and invoke set-role via the wrapped subcommand path.
	srv := burrow.NewServer(session.New(), burrowtest.StubApp("csrf"), burrowtest.StubApp("staticfiles"), New[EmptyProfile]())
	cmd := &cli.Command{
		Name:     "test",
		Flags:    srv.Flags(nil),
		Commands: srv.CLICommands(),
	}
	err := cmd.Run(ctx, []string{"test", "--database-dsn", dsn, "set-role", "alice", "admin"})
	require.NoError(t, err, "set-role subcommand must succeed when wrapped by Server.CLICommands")

	// Verify alice's role is admin in the database.
	db, err := burrow.OpenDB(ctx, dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	require.NoError(t, den.Register(ctx, db, New[EmptyProfile]().Documents()...))
	repo := NewRepository[EmptyProfile](db)
	user, err := repo.GetUserByUsername(ctx, "alice")
	require.NoError(t, err)
	assert.Equal(t, RoleAdmin, user.Role)
}

// --- Admin tests ---

func TestAdminNavItems(t *testing.T) {
	app := &App[EmptyProfile]{}
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
func newTestApp(t *testing.T) (*App[EmptyProfile], *Repository[EmptyProfile]) {
	t.Helper()
	db := openTestDB(t)
	reg := registry.New()
	sessionApp := session.New()
	registry.Add(reg, sessionApp)
	// Stub auth's declared dependencies; tests here exercise handler-level
	// behaviour only, not csrf/staticfiles wiring.
	registry.Add(reg, burrowtest.StubApp("csrf"))
	registry.Add(reg, burrowtest.StubApp("staticfiles"))
	app := New[EmptyProfile]()
	registry.Add(reg, app)
	appCfg := &burrow.AppConfig{DB: db, Registry: reg, Config: &burrow.Config{}}

	cmd := &cli.Command{
		Name:  "test",
		Flags: append(sessionApp.Flags(nil), app.Flags(nil)...),
		Action: func(_ context.Context, cmd *cli.Command) error {
			if err := sessionApp.Configure(appCfg, cmd); err != nil {
				return err
			}
			return app.Configure(appCfg, cmd)
		},
	}
	require.NoError(t, cmd.Run(t.Context(), []string{
		"test",
		"--auth-webauthn-rp-id", "localhost",
		"--auth-webauthn-rp-display-name", "Test",
		"--auth-webauthn-rp-origin", "http://localhost",
	}))
	t.Cleanup(func() { _ = app.Shutdown(context.Background()) })

	app.config = &Config{}
	return app, app.repo
}

func adminUserRouter(app *App[EmptyProfile]) *chi.Mux {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// AdminRoutes self-gates with RequireAdmin(); inject a stub
			// admin user so the handler-level tests can exercise behaviour
			// without re-implementing the admin coordinator's setup.
			admin := &User[EmptyProfile]{Username: "admin-fixture", Role: RoleAdmin, IsActive: true}
			admin.ID = "admin-fixture-id"
			rctx := WithUser(r.Context(), admin)
			rctx = burrowtest.ErrorExecContext(rctx)
			next.ServeHTTP(w, r.WithContext(rctx))
		})
	})
	app.AdminRoutes(router)
	return router
}

func TestAdminUpdateUser(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)

	router := adminUserRouter(app)

	body := strings.NewReader("name=Alice+Wonder&bio=Hello+World&email=alice%40example.com&role=admin")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/users/"+user.ID, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/admin/users", rec.Header().Get("Location"))

	got, err := repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Email)
	assert.Equal(t, "alice@example.com", *got.Email)
	assert.Equal(t, RoleAdmin, got.Role)
}

func TestAdminUpdateUserContinueEditing(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)

	router := adminUserRouter(app)

	body := strings.NewReader("name=Alice&role=user&_continue=1")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/users/"+user.ID, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/admin/users/"+user.ID, rec.Header().Get("Location"))
}

func TestAdminUpdateUserNotFound(t *testing.T) {
	app, _ := newTestApp(t)
	router := adminUserRouter(app)

	body := strings.NewReader("name=Test&role=user")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/users/999", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminUpdateUserLastAdminDemotion(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	admin, err := repo.CreateUser(ctx, "admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, admin.ID, RoleAdmin))

	router := adminUserRouter(app)

	body := strings.NewReader("name=Admin&role=user")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/users/"+admin.ID, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, "should reject demotion of last admin")

	got, err := repo.GetUserByID(ctx, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, RoleAdmin, got.Role, "role should remain admin")
}

func TestAdminUpdateUserDemoteNonLastAdmin(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	admin1, err := repo.CreateUser(ctx, "admin1")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, admin1.ID, RoleAdmin))

	admin2, err := repo.CreateUser(ctx, "admin2")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, admin2.ID, RoleAdmin))

	router := adminUserRouter(app)

	body := strings.NewReader("name=Admin2&role=user")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/users/"+admin2.ID, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code, "should allow demotion when multiple admins exist")

	got, err := repo.GetUserByID(ctx, admin2.ID)
	require.NoError(t, err)
	assert.Equal(t, RoleUser, got.Role)
}

// --- Admin invite handler tests ---

func TestAdminCreateInvite(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()
	user, _ := repo.CreateUser(ctx, "admin")

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

	// Without email config, the registration URL is stashed in the session
	// for the list page to render in a copyable input + button.
	sessionValues := session.GetValues(req)
	require.NotNil(t, sessionValues)
	storedURL, ok := sessionValues[sessionKeyInviteCreatedURL].(string)
	require.True(t, ok, "expected created-url in session")
	assert.Contains(t, storedURL, "/auth/register?invite=")
}

func TestAdminCreateInviteNoAuth(t *testing.T) {
	app, _ := newTestApp(t)

	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrowtest.ErrorExecContext(r.Context())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Post("/admin/invites", burrow.Handle(app.handleCreateInvite))

	body := strings.NewReader(`email=test@example.com`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/invites", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRevokeInviteNonExistentID(t *testing.T) {
	_, repo := newTestApp(t)

	router := chi.NewRouter()
	router.Delete("/admin/invites/{id}/revoke", burrow.Handle(revokeInviteHandler(repo)))

	// With string IDs, any non-empty string is a valid ID format.
	// Revoking a non-existent invite is a no-op that redirects.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/admin/invites/nonexistent/revoke", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Non-HTMX: SmartRedirect returns 303 SeeOther.
	assert.Equal(t, http.StatusSeeOther, rec.Code)
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/admin/invites/"+invite.ID+"/revoke", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/admin/invites", rec.Header().Get("HX-Redirect"))

	// Verify invite was deleted.
	invites, err := repo.ListInvites(ctx)
	require.NoError(t, err)
	assert.Empty(t, invites)
}

// --- Admin delete user tests ---

func TestDeleteUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)

	err = repo.DeleteUser(ctx, user.ID)
	require.NoError(t, err)

	// Deleted user should not appear in ListUsers.
	users, err := repo.ListUsers(ctx)
	require.NoError(t, err)
	assert.Empty(t, users)

	// GetUserByID should also not find the deleted user.
	_, err = repo.GetUserByID(ctx, user.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestAdminDeleteUserSuccess(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	admin, err := repo.CreateUser(ctx, "admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, admin.ID, RoleAdmin))
	admin.Role = RoleAdmin

	target, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rctx := WithUser(r.Context(), admin)
			rctx = burrowtest.ErrorExecContext(rctx)
			next.ServeHTTP(w, r.WithContext(rctx))
		})
	})
	app.AdminRoutes(router)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/users/"+target.ID, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/admin/users", rec.Header().Get("Location"))

	// Verify user was deleted.
	users, err := repo.ListUsers(ctx)
	require.NoError(t, err)
	assert.Len(t, users, 1, "only the admin should remain")
}

func TestAdminDeleteUserNotFound(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	admin, err := repo.CreateUser(ctx, "admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, admin.ID, RoleAdmin))
	admin.Role = RoleAdmin

	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rctx := WithUser(r.Context(), admin)
			rctx = burrowtest.ErrorExecContext(rctx)
			next.ServeHTTP(w, r.WithContext(rctx))
		})
	})
	app.AdminRoutes(router)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/users/999", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// DeleteUser silently succeeds when ID doesn't exist.
	assert.Equal(t, http.StatusSeeOther, rec.Code)
}

// --- PurgeOrphanedUsers tests ---

func TestPurgeOrphanedUsersDeletesUsersWithoutCredentials(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	// Create a user with no credentials (simulates abandoned registration).
	orphan, err := repo.CreateUser(ctx, "orphan")
	require.NoError(t, err)

	// Backdate the user's created_at so it qualifies for cleanup.
	orphanReloaded, err := repo.GetUserByID(ctx, orphan.ID)
	require.NoError(t, err)
	orphanReloaded.CreatedAt = time.Now().Add(-10 * time.Minute)
	require.NoError(t, den.Save(ctx, db, orphanReloaded))

	// Create a user WITH credentials (should be kept).
	legit, err := repo.CreateUser(ctx, "legit")
	require.NoError(t, err)
	legitReloaded, err := repo.GetUserByID(ctx, legit.ID)
	require.NoError(t, err)
	legitReloaded.CreatedAt = time.Now().Add(-10 * time.Minute)
	require.NoError(t, den.Save(ctx, db, legitReloaded))
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
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	// Create a user with no credentials (just created — not yet orphaned).
	_, err := repo.CreateUser(ctx, "newuser")
	require.NoError(t, err)

	// Purge with 5-minute threshold — should not delete the recent user.
	purged, err := repo.PurgeOrphanedUsers(ctx, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 0, purged)
}

// --- SetUserActive / Deactivate / Activate tests ---

func TestSetUserActive(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
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

// userActionRouter creates a chi router with a POST handler and user context.
func userActionRouter(handler burrow.HandlerFunc, user *User[EmptyProfile]) *chi.Mux {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rctx := WithUser(r.Context(), user)
			rctx = burrowtest.ErrorExecContext(rctx)
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

	adminUser, err := repo.CreateUser(ctx, "admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, adminUser.ID, RoleAdmin))

	target, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)

	router := userActionRouter(deactivateUserHandler(repo), adminUser)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/users/"+target.ID+"/deactivate", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/admin/users", rec.Header().Get("HX-Redirect"))

	updated, err := repo.GetUserByID(ctx, target.ID)
	require.NoError(t, err)
	assert.False(t, updated.IsActive)
}

func TestActivateUserSuccess(t *testing.T) {
	_, repo := newTestApp(t)
	ctx := context.Background()

	adminUser, err := repo.CreateUser(ctx, "admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, adminUser.ID, RoleAdmin))

	target, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserActive(ctx, target.ID, false))

	router := userActionRouter(activateUserHandler(repo), adminUser)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/users/"+target.ID+"/activate", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/admin/users", rec.Header().Get("HX-Redirect"))

	updated, err := repo.GetUserByID(ctx, target.ID)
	require.NoError(t, err)
	assert.True(t, updated.IsActive)
}

// --- Profile-type guard tests ---

type plainProfileFixture struct {
	Name string
}

type docEmbeddingProfileFixture struct {
	document.Base
	Name string
}

func TestProfileTypeGuard_AcceptsEmptyProfile(t *testing.T) {
	require.NoError(t, validateProfileType[EmptyProfile]())
}

func TestProfileTypeGuard_AcceptsPlainStruct(t *testing.T) {
	require.NoError(t, validateProfileType[plainProfileFixture]())
}

func TestProfileTypeGuard_RejectsDocumentEmbedder(t *testing.T) {
	err := validateProfileType[docEmbeddingProfileFixture]()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Profile type")
	assert.Contains(t, err.Error(), "document.Base")
	assert.Contains(t, err.Error(), "docEmbeddingProfileFixture")
}

func TestProfileTypeGuard_RejectsPointerToDocument(t *testing.T) {
	err := validateProfileType[*docEmbeddingProfileFixture]()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "document.Base")
}

// --- WithAuthLayout option tests ---

func TestWithAuthLayoutOption(t *testing.T) {
	app := New[EmptyProfile](WithAuthLayout[EmptyProfile]("test/layout"))
	assert.Equal(t, "test/layout", app.authLayout, "authLayout should be set via WithAuthLayout option")
}

func TestPublicAuthRoutesUseAuthLayout(t *testing.T) {
	// Set up a mock renderer that captures the layout from context.
	var capturedLayout string
	mockR := &layoutCapturingRenderer{capturedLayout: &capturedLayout}

	app := &App[EmptyProfile]{
		renderer:   mockR,
		config:     &Config{LoginRedirect: "/"},
		authLayout: "test/auth-layout",
	}

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
	assert.Equal(t, "test/auth-layout", *mockR.capturedLayout, "layout should be the auth layout")
}

func TestAuthenticatedRoutesKeepGlobalLayout(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)

	// Set up a mock renderer that captures the layout from context.
	var capturedLayout string
	mockR := &layoutCapturingRenderer{capturedLayout: &capturedLayout}

	app := &App[EmptyProfile]{
		repo:       repo,
		renderer:   mockR,
		config:     &Config{LoginRedirect: "/"},
		authLayout: "test/auth-layout",
	}

	// Create a user so the credentials handler can look up credentials.
	user, err := repo.CreateUser(context.Background(), "alice")
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

	app := &App[EmptyProfile]{
		renderer: mockR,
		config:   &Config{LoginRedirect: "/"},
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

func (m *layoutCapturingRenderer) VerifyEmailSuccessPage(w http.ResponseWriter, r *http.Request) error {
	lay := burrow.Layout(r.Context())
	*m.capturedLayout = lay
	return burrow.Text(w, http.StatusOK, "verify-success")
}

func (m *layoutCapturingRenderer) VerifyEmailErrorPage(w http.ResponseWriter, r *http.Request, _ string) error {
	lay := burrow.Layout(r.Context())
	*m.capturedLayout = lay
	return burrow.Text(w, http.StatusBadRequest, "verify-error")
}

// --- Static files tests ---

func TestStaticFS(t *testing.T) {
	app := &App[EmptyProfile]{}
	prefix, fsys := app.StaticFS()

	assert.Equal(t, "auth", prefix)
	require.NotNil(t, fsys)

	f, err := fsys.Open("webauthn.js")
	require.NoError(t, err, "webauthn.js should exist in static FS")
	_ = f.Close()
}

// --- Model tests ---

func TestUserWebAuthnMethods(t *testing.T) {
	user := &User[EmptyProfile]{Username: "alice"}
	user.ID = "01ABCDEFGH123456789012"

	assert.Equal(t, "alice", user.WebAuthnName())
	assert.Equal(t, "alice", user.WebAuthnDisplayName(), "auth-core returns Username; richer display lives in Profile")
	assert.NotEmpty(t, user.WebAuthnID())
	assert.Empty(t, user.WebAuthnIcon())
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
	app := &App[EmptyProfile]{}
	user := &User[EmptyProfile]{Username: "alice"}
	user.ID = "01ABCDEFGH123456789014"

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	ctx := WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	fm := app.RequestFuncMap(req.Context())

	currentUserFunc := fm["currentUser"].(func() *User[EmptyProfile])
	assert.Equal(t, user, currentUserFunc())

	isAuthFunc := fm["isAuthenticated"].(func() bool)
	assert.True(t, isAuthFunc())
}

func TestRequestFuncMapUnauthenticated(t *testing.T) {
	app := &App[EmptyProfile]{}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

	fm := app.RequestFuncMap(req.Context())

	currentUserFunc := fm["currentUser"].(func() *User[EmptyProfile])
	assert.Nil(t, currentUserFunc())

	isAuthFunc := fm["isAuthenticated"].(func() bool)
	assert.False(t, isAuthFunc())
}

// --- Option functions ---

func TestWithRendererOption(t *testing.T) {
	r := &mockRenderer{}
	app := New[EmptyProfile](WithRenderer[EmptyProfile](r))
	assert.Equal(t, r, app.renderer)
}

func TestWithLogoComponentOption(t *testing.T) {
	logo := template.HTML(`<img src="logo.png"/>`)
	app := New[EmptyProfile](WithLogoComponent[EmptyProfile](logo))
	assert.Equal(t, logo, app.logo)
}

func TestWithEmailServiceOption(t *testing.T) {
	svc := &mockEmailService{}
	app := New[EmptyProfile](WithEmailService[EmptyProfile](svc))
	assert.Equal(t, svc, app.emailService)
}

func TestTranslationFS(t *testing.T) {
	app := &App[EmptyProfile]{}
	fsys := app.TranslationFS()
	require.NotNil(t, fsys)
}

// TestAdminNavItemsResolveViaLabelAsKey pins that each AdminNavItem Label
// also exists as an i18n message ID in both translation files — so
// buildNavLinks's i18n.T(ctx, item.Label) lookup hits a translation instead
// of rendering the raw English string. Catches rename drift between
// NavItem labels, template lookups, and translation keys.
func TestAdminNavItemsResolveViaLabelAsKey(t *testing.T) {
	bundle, err := i18n.NewTestBundle(translationFS)
	require.NoError(t, err)

	items := (&App[EmptyProfile]{}).AdminNavItems()
	require.NotEmpty(t, items)

	expectedDE := map[string]string{
		"Users":   "Benutzer",
		"Invites": "Einladungen",
	}

	for _, item := range items {
		t.Run(item.Label, func(t *testing.T) {
			ctxEN := bundle.WithLocale(context.Background(), "en")
			assert.Equal(t, item.Label, i18n.T(ctxEN, item.Label), "English lookup must resolve to the Label itself")

			ctxDE := bundle.WithLocale(context.Background(), "de")
			assert.Equal(t, expectedDE[item.Label], i18n.T(ctxDE, item.Label), "German lookup must resolve to the translated string, not the raw Label")
		})
	}
}

// TestUserEditFormLabelsTranslate pins that the User edit form's labels
// resolve to German via the Label-as-key convention. Catches rename drift
// between User struct verbose tags, WithChoices Labels, and translation TOMLs.
func TestUserEditFormLabelsTranslate(t *testing.T) {
	bundle, err := i18n.NewTestBundle(translationFS)
	require.NoError(t, err)

	user := &User[EmptyProfile]{Role: RoleUser}
	ctxDE := bundle.WithLocale(context.Background(), "de")
	f := forms.FromModel(user, userFormOpts[EmptyProfile]()...).WithContext(ctxDE)

	expected := map[string]string{
		"Username": "Benutzername",
		"Email":    "E-Mail",
		"Role":     "Rolle",
		"IsActive": "Aktiv",
	}
	for name, want := range expected {
		bf, ok := f.Field(name)
		require.True(t, ok, "field %s not found", name)
		assert.Equal(t, want, bf.Label, "BoundField %s must translate via Label-as-key", name)
	}

	roleField, ok := f.Field("Role")
	require.True(t, ok)
	require.NotEmpty(t, roleField.Choices, "Role field must carry choices")
	choiceByValue := map[string]string{}
	for _, c := range roleField.Choices {
		choiceByValue[c.Value] = c.Label
	}
	assert.Equal(t, "Benutzer", choiceByValue[RoleUser], "Choice label for RoleUser must translate")
	assert.Equal(t, "Administrator", choiceByValue[RoleAdmin], "Choice label for RoleAdmin must translate")
}

func TestTemplateFS(t *testing.T) {
	app := &App[EmptyProfile]{}
	fsys := app.TemplateFS()
	require.NotNil(t, fsys)
}

func TestFuncMap(t *testing.T) {
	app := &App[EmptyProfile]{}
	fm := app.FuncMap()
	require.NotNil(t, fm)

	// Test credName function.
	credNameFunc, ok := fm["credName"].(func(Credential) string)
	require.True(t, ok)
	assert.Equal(t, "My Key", credNameFunc(Credential{Name: "My Key"}))
	assert.Equal(t, "Passkey", credNameFunc(Credential{Name: ""}))

	// Test deref function.
	derefFunc, ok := fm["deref"].(func(*string) string)
	require.True(t, ok)
	s := "hello"
	assert.Equal(t, "hello", derefFunc(&s))
	assert.Empty(t, derefFunc(nil))
}

func TestShutdown(t *testing.T) {
	// Shutdown with nil cancelCleanup should not panic.
	app := &App[EmptyProfile]{}
	err := app.Shutdown(context.Background())
	require.NoError(t, err)

	// Shutdown with a real cancel function.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app2 := &App[EmptyProfile]{cancelCleanup: cancel}
	err = app2.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestShutdownMultipleCalls(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	app := &App[EmptyProfile]{cancelCleanup: cancel}

	// First call should be fine.
	assert.NoError(t, app.Shutdown(context.Background()))
	// Second call should also be safe (cancel is idempotent).
	assert.NoError(t, app.Shutdown(context.Background()))
}

func TestRepoAccessor(t *testing.T) {
	repo := &Repository[EmptyProfile]{}
	app := &App[EmptyProfile]{repo: repo}
	assert.Same(t, repo, app.Repo())
}

func TestAuthMiddlewareWithValidUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)

	user, err := repo.CreateUser(context.Background(), "alice")
	require.NoError(t, err)

	app := &App[EmptyProfile]{repo: repo, config: &Config{LoginRedirect: "/dashboard"}}

	var gotUser *User[EmptyProfile]
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = CurrentUser[EmptyProfile](r.Context())
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

func TestAuthMiddlewareWithInactiveUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)

	user, err := repo.CreateUser(context.Background(), "inactive")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserActive(context.Background(), user.ID, false))

	app := &App[EmptyProfile]{repo: repo, config: &Config{LoginRedirect: "/dashboard"}}

	var gotUser *User[EmptyProfile]
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = CurrentUser[EmptyProfile](r.Context())
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
	repo := NewRepository[EmptyProfile](db)

	app := &App[EmptyProfile]{repo: repo, config: &Config{LoginRedirect: "/dashboard"}}

	var gotUser *User[EmptyProfile]
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = CurrentUser[EmptyProfile](r.Context())
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
		gotLogo = Logo(r.Context())
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

func TestRequestFuncMapAuthLogo(t *testing.T) {
	app := &App[EmptyProfile]{}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	logo := template.HTML(`<span>Logo</span>`)
	ctx := WithLogo(req.Context(), logo)
	req = req.WithContext(ctx)

	fm := app.RequestFuncMap(req.Context())
	logoFunc := fm["authLogo"].(func() template.HTML)
	assert.Equal(t, logo, logoFunc())
}

func TestNewWithMultipleOptions(t *testing.T) {
	r := &mockRenderer{}
	logo := template.HTML(`<span>Logo</span>`)
	emailSvc := &mockEmailService{}

	app := New(
		WithRenderer[EmptyProfile](r),
		WithLogoComponent[EmptyProfile](logo),
		WithEmailService[EmptyProfile](emailSvc),
		WithAuthLayout[EmptyProfile]("custom/auth-layout"),
	)

	assert.Equal(t, r, app.renderer)
	assert.Equal(t, logo, app.logo)
	assert.Equal(t, emailSvc, app.emailService)
	assert.Equal(t, "custom/auth-layout", app.authLayout)
}

func TestDependencies(t *testing.T) {
	app := &App[EmptyProfile]{}
	assert.Equal(t, []string{"session", "csrf", "staticfiles"}, app.Dependencies())
}

func TestAdminRoutes(t *testing.T) {
	app, _ := newTestApp(t)

	// AdminRoutes should not panic when called.
	router := chi.NewRouter()
	app.AdminRoutes(router)
}

func TestRoutesWithLogoMiddleware(t *testing.T) {
	app := &App[EmptyProfile]{
		renderer: &mockRenderer{},
		config:   &Config{LoginRedirect: "/"},
		logo:     template.HTML(`<img src="logo.png"/>`),
	}

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
	reg := registry.New()
	registry.Add(reg, session.New())
	registry.Add(reg, burrowtest.StubApp("csrf"))
	registry.Add(reg, burrowtest.StubApp("staticfiles"))
	app := New[EmptyProfile]()
	registry.Add(reg, app)
	appCfg := &burrow.AppConfig{DB: db, Registry: reg, Config: &burrow.Config{}}

	// Build a CLI command that sets the flags and calls Configure.
	cliCmd := &cli.Command{
		Name:  "test",
		Flags: app.Flags(nil),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return app.Configure(appCfg, cmd)
		},
	}

	err := cliCmd.Run(context.Background(), []string{
		"test",
		"--auth-webauthn-rp-id", "localhost",
		"--auth-webauthn-rp-display-name", "Test App",
		"--auth-webauthn-rp-origin", "http://localhost:8080",
		"--auth-login-redirect", "/home",
		"--auth-logout-redirect", "/goodbye",
	})
	require.NoError(t, err)

	// Verify configuration was applied.
	require.NotNil(t, app.config)
	assert.Equal(t, "/home", app.config.LoginRedirect)
	assert.Equal(t, "/goodbye", app.config.LogoutRedirect)
	require.NotNil(t, app.webauthn)
	require.NotNil(t, app.recovery)

	// Start launches the background cleanup goroutine.
	require.NoError(t, app.Start(nil))
	require.NotNil(t, app.cancelCleanup)

	// Clean up.
	require.NoError(t, app.Shutdown(context.Background()))
}

func TestConfigureWithDefaultOrigin(t *testing.T) {
	db := openTestDB(t)
	reg := registry.New()
	registry.Add(reg, session.New())
	registry.Add(reg, burrowtest.StubApp("csrf"))
	registry.Add(reg, burrowtest.StubApp("staticfiles"))
	app := New[EmptyProfile]()
	registry.Add(reg, app)
	appCfg := &burrow.AppConfig{DB: db, Registry: reg, Config: &burrow.Config{}}

	cliCmd := &cli.Command{
		Name:  "test",
		Flags: app.Flags(nil),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return app.Configure(appCfg, cmd)
		},
	}

	// No --webauthn-rp-origin set, should fallback to base URL.
	err := cliCmd.Run(context.Background(), []string{
		"test",
		"--auth-webauthn-rp-id", "localhost",
		"--auth-webauthn-rp-display-name", "Test App",
	})
	require.NoError(t, err)
	require.NotNil(t, app.config)

	require.NoError(t, app.Shutdown(context.Background()))
}

func TestCleanupOrphanedUsersStopsOnCancel(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	app := &App[EmptyProfile]{repo: repo}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		app.backgroundCleanup(ctx)
		close(done)
	}()

	// Cancel immediately to test the context cancellation path.
	cancel()

	select {
	case <-done:
		// Goroutine exited cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("backgroundCleanup did not stop within 2 seconds")
	}
}

func TestCLISetRoleNoArgs(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	app := &App[EmptyProfile]{repo: repo}
	setRoleCmd := findSetRoleCmd(t, app)

	cliCmd := &cli.Command{
		Name:     "test",
		Action:   func(ctx context.Context, cmd *cli.Command) error { return nil },
		Commands: []*cli.Command{setRoleCmd},
	}
	err := cliCmd.Run(context.Background(), []string{"test", "set-role"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "username and role are required")
}

func TestCLISetRoleUserNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	app := &App[EmptyProfile]{repo: repo}
	setRoleCmd := findSetRoleCmd(t, app)

	cliCmd := &cli.Command{
		Name:     "test",
		Action:   func(ctx context.Context, cmd *cli.Command) error { return nil },
		Commands: []*cli.Command{setRoleCmd},
	}
	err := cliCmd.Run(context.Background(), []string{"test", "set-role", "nonexistent", "admin"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCLICreateInviteNoArgs(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	app := &App[EmptyProfile]{repo: repo}
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

	user, _ := repo.CreateUser(ctx, "admin")

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
	u := &User[EmptyProfile]{Username: "test"}
	u.ID = "test-user-1"
	req = requestWithSession(req, u)
	rec := httptest.NewRecorder()

	err := app.handleCreateInvite(rec, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email is required")
}

// --- backgroundCleanup: ticker path ---

func TestBackgroundCleanupPurgesOrphanedUsers(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	// Create an orphaned user (no credentials, created > 5 min ago).
	user, err := repo.CreateUser(ctx, "orphan")
	require.NoError(t, err)

	// Backdate the user's created_at to make it eligible for purge.
	userReloaded, err := repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	userReloaded.CreatedAt = time.Now().Add(-10 * time.Minute)
	require.NoError(t, den.Save(ctx, db, userReloaded))

	// Also create an expired email verification token.
	err = repo.CreateEmailVerificationToken(ctx, user.ID, "expired-hash", time.Now().Add(-time.Hour))
	require.NoError(t, err)

	// Run cleanup logic directly (the same code backgroundCleanup executes on tick).
	purged, err := repo.PurgeOrphanedUsers(ctx, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 1, purged)

	err = repo.DeleteExpiredEmailVerificationTokens(ctx)
	require.NoError(t, err)
}

// --- set-role: repo not initialized ---

func TestSetRoleRepoNotInitialized(t *testing.T) {
	app := &App[EmptyProfile]{repo: nil}

	cliCmd := &cli.Command{
		Name:      "test-set-role",
		ArgsUsage: "<username> <role>",
		Action:    app.setRoleAction,
	}

	err := cliCmd.Run(context.Background(), []string{"test-set-role", "alice", "admin"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth app not initialized")
}

func TestSetRoleActionInvalidArgs(t *testing.T) {
	app := &App[EmptyProfile]{repo: nil}

	cliCmd := &cli.Command{
		Name:      "test-set-role",
		ArgsUsage: "<username> <role>",
		Action:    app.setRoleAction,
	}

	err := cliCmd.Run(context.Background(), []string{"test-set-role", "alice"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "username and role are required")
}

func TestSetRoleActionInvalidRole(t *testing.T) {
	app := &App[EmptyProfile]{repo: nil}

	cliCmd := &cli.Command{
		Name:      "test-set-role",
		ArgsUsage: "<username> <role>",
		Action:    app.setRoleAction,
	}

	err := cliCmd.Run(context.Background(), []string{"test-set-role", "alice", "robot"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid role "robot"`)
}

// --- createInviteAction: repo not initialized ---

func TestCreateInviteActionRepoNotInitialized(t *testing.T) {
	app := &App[EmptyProfile]{repo: nil}
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
	err := cliCmd.Run(context.Background(), []string{"test", "create-invite", "test@example.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth app not initialized")
}

// --- createInviteAction: success with globalConfig ---

func TestCreateInviteActionSuccess(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	app := &App[EmptyProfile]{
		repo:         repo,
		globalConfig: &burrow.Config{},
	}
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
	err := cliCmd.Run(context.Background(), []string{"test", "create-invite", "invite@example.com"})
	require.NoError(t, err)
}

// --- authMiddleware: admin user sets IsAdmin true ---

func TestAuthMiddlewareWithAdminUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)

	user, err := repo.CreateUser(context.Background(), "admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(context.Background(), user.ID, RoleAdmin))

	app := &App[EmptyProfile]{repo: repo, config: &Config{LoginRedirect: "/dashboard"}}

	var gotUser *User[EmptyProfile]
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = CurrentUser[EmptyProfile](r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req = session.Inject(req, map[string]any{"user_id": user.ID})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, gotUser)
	assert.Equal(t, "admin", gotUser.Username)
	assert.True(t, gotUser.IsAdmin(), "admin user should have admin role")
}

// --- deactivateUserHandler: invalid ID ---

func TestDeactivateUserInvalidID(t *testing.T) {
	_, repo := newTestApp(t)
	adminUser := &User[EmptyProfile]{Username: "admin", Role: RoleAdmin, IsActive: true}
	adminUser.ID = "admin-id-1"

	router := userActionRouter(deactivateUserHandler(repo), adminUser)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/users/nonexistent/deactivate", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// chi returns 405 or the handler returns a 400. Let's check the response is an error.
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

// --- activateUserHandler: invalid ID ---

func TestActivateUserInvalidID(t *testing.T) {
	_, repo := newTestApp(t)
	adminUser := &User[EmptyProfile]{Username: "admin", Role: RoleAdmin, IsActive: true}
	adminUser.ID = "admin-id-2"

	router := userActionRouter(activateUserHandler(repo), adminUser)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/users/nonexistent/activate", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusOK, rec.Code)
}

// --- deactivateUserHandler: DB error on SetUserActive ---

func TestDeactivateUserDBError(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	adminUser, err := repo.CreateUser(ctx, "admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, adminUser.ID, RoleAdmin))

	target, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)

	// Close the DB to force an error on SetUserActive.
	require.NoError(t, db.Close())

	router := userActionRouter(deactivateUserHandler(repo), adminUser)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/users/"+target.ID+"/deactivate", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- activateUserHandler: DB error on SetUserActive ---

func TestActivateUserDBError(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	adminUser, err := repo.CreateUser(ctx, "admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, adminUser.ID, RoleAdmin))

	target, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserActive(ctx, target.ID, false))

	// Close the DB to force an error on SetUserActive.
	require.NoError(t, db.Close())

	router := userActionRouter(activateUserHandler(repo), adminUser)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/users/"+target.ID+"/activate", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
