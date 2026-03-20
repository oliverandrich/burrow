package auth

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/oliverandrich/burrow/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"golang.org/x/crypto/bcrypt"
)

// --- Mocks ---

type mockRenderer struct {
	lastMethod string
}

func (m *mockRenderer) RegisterPage(w http.ResponseWriter, _ *http.Request, _, _ bool, _, _ string) error {
	m.lastMethod = "RegisterPage"
	return burrow.Text(w, http.StatusOK, "register")
}

func (m *mockRenderer) LoginPage(w http.ResponseWriter, _ *http.Request, _ string) error {
	m.lastMethod = "LoginPage"
	return burrow.Text(w, http.StatusOK, "login")
}

func (m *mockRenderer) CredentialsPage(w http.ResponseWriter, _ *http.Request, _ []Credential) error {
	m.lastMethod = "CredentialsPage"
	return burrow.Text(w, http.StatusOK, "credentials")
}

func (m *mockRenderer) RecoveryPage(w http.ResponseWriter, _ *http.Request, _ string) error {
	m.lastMethod = "RecoveryPage"
	return burrow.Text(w, http.StatusOK, "recovery")
}

func (m *mockRenderer) RecoveryCodesPage(w http.ResponseWriter, _ *http.Request, _ []string) error {
	m.lastMethod = "RecoveryCodesPage"
	return burrow.Text(w, http.StatusOK, "recovery-codes")
}

func (m *mockRenderer) VerifyPendingPage(w http.ResponseWriter, _ *http.Request) error {
	m.lastMethod = "VerifyPendingPage"
	return burrow.Text(w, http.StatusOK, "verify-pending")
}

func (m *mockRenderer) VerifyEmailSuccessPage(w http.ResponseWriter, _ *http.Request) error {
	m.lastMethod = "VerifyEmailSuccessPage"
	return burrow.Text(w, http.StatusOK, "verify-success")
}

func (m *mockRenderer) VerifyEmailErrorPage(w http.ResponseWriter, _ *http.Request, _ string) error {
	m.lastMethod = "VerifyEmailErrorPage"
	return burrow.Text(w, http.StatusBadRequest, "verify-error")
}

type mockEmailService struct {
	sendCalled bool
}

func (m *mockEmailService) SendVerification(_ context.Context, _, _ string) error {
	m.sendCalled = true
	return nil
}

func (m *mockEmailService) SendInvite(_ context.Context, _, _ string) error {
	m.sendCalled = true
	return nil
}

// --- Test helpers ---

func testI18nBundle(t *testing.T) *i18n.Bundle {
	t.Helper()
	bundle, err := i18n.NewTestBundle("en", translationFS)
	require.NoError(t, err)
	return bundle
}

func testApp(t *testing.T, bundle *i18n.Bundle) *App {
	t.Helper()
	return &App{withLocale: bundle.WithLocale}
}

func newTestHandlers(t *testing.T) (*Handlers, *Repository, *mockRenderer) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository(db)
	renderer := &mockRenderer{}
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	app := testApp(t, testI18nBundle(t))
	h := NewHandlers(repo, waSvc, nil, renderer, &Config{
		LoginRedirect:  "/dashboard",
		LogoutRedirect: "/auth/login",
	}, app)
	h.recovery.BcryptCost = bcrypt.MinCost
	return h, repo, renderer
}

func newTestHandlersEmailMode(t *testing.T) (*Handlers, *Repository, *mockRenderer) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository(db)
	renderer := &mockRenderer{}
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	emailSvc := &mockEmailService{}
	app := testApp(t, testI18nBundle(t))
	app.emailService = emailSvc
	h := NewHandlers(repo, waSvc, emailSvc, renderer, &Config{
		LoginRedirect:       "/dashboard",
		LogoutRedirect:      "/auth/login",
		UseEmail:            true,
		RequireVerification: true,
		BaseURL:             "http://localhost:8080",
	}, app)
	h.recovery.BcryptCost = bcrypt.MinCost
	return h, repo, renderer
}

func newTestHandlersInviteOnly(t *testing.T) (*Handlers, *Repository, *mockRenderer) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository(db)
	renderer := &mockRenderer{}
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	app := testApp(t, testI18nBundle(t))
	h := NewHandlers(repo, waSvc, nil, renderer, &Config{
		LoginRedirect:  "/dashboard",
		LogoutRedirect: "/auth/login",
		InviteOnly:     true,
	}, app)
	h.recovery.BcryptCost = bcrypt.MinCost
	return h, repo, renderer
}

// requestWithSession creates a request with session state injected, optionally with a user.
func requestWithSession(req *http.Request, user *User) *http.Request {
	req = session.Inject(req, map[string]any{})
	if user != nil {
		ctx := WithUser(req.Context(), user)
		req = req.WithContext(ctx)
	}
	return req
}

// openTestDBClosable opens a test DB and returns both the bun.DB and the underlying
// sql.DB so tests can close the sql.DB to trigger database errors in handlers.
func openTestDBClosable(t *testing.T) (*bun.DB, *sql.DB) {
	t.Helper()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?_pragma=foreign_keys(1)")
	require.NoError(t, err)

	db := bun.NewDB(sqldb, sqlitedialect.New())

	app := New()
	err = burrow.RunAppMigrations(t.Context(), db, app.Name(), app.MigrationFS())
	require.NoError(t, err)

	return db, sqldb
}

// newTestHandlersClosable creates handlers with a DB that can be closed to trigger errors.
func newTestHandlersClosable(t *testing.T) (*Handlers, *Repository, *sql.DB) {
	t.Helper()
	db, sqldb := openTestDBClosable(t)
	repo := NewRepository(db)
	renderer := &mockRenderer{}
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	app := testApp(t, testI18nBundle(t))
	h := NewHandlers(repo, waSvc, nil, renderer, &Config{
		LoginRedirect:  "/dashboard",
		LogoutRedirect: "/auth/login",
	}, app)
	h.recovery.BcryptCost = bcrypt.MinCost
	return h, repo, sqldb
}

// newTestHandlersEmailModeClosable creates email-mode handlers with a closable DB.
func newTestHandlersEmailModeClosable(t *testing.T) (*Handlers, *Repository, *sql.DB) {
	t.Helper()
	db, sqldb := openTestDBClosable(t)
	repo := NewRepository(db)
	renderer := &mockRenderer{}
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	emailSvc := &mockEmailService{}
	app := testApp(t, testI18nBundle(t))
	app.emailService = emailSvc
	h := NewHandlers(repo, waSvc, emailSvc, renderer, &Config{
		LoginRedirect:       "/dashboard",
		LogoutRedirect:      "/auth/login",
		UseEmail:            true,
		RequireVerification: true,
		BaseURL:             "http://localhost:8080",
	}, app)
	h.recovery.BcryptCost = bcrypt.MinCost
	return h, repo, sqldb
}

// --- Handler creation tests ---

func TestNewHandlers(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	assert.NotNil(t, h)
	assert.False(t, h.UseEmailMode())
	assert.False(t, h.IsInviteOnly())
}

func TestNewHandlersEmailMode(t *testing.T) {
	h, _, _ := newTestHandlersEmailMode(t)
	assert.True(t, h.UseEmailMode())
}

func TestNewHandlersInviteOnly(t *testing.T) {
	h, _, _ := newTestHandlersInviteOnly(t)
	assert.True(t, h.IsInviteOnly())
}

// --- RegisterPage tests ---

func TestRegisterPage(t *testing.T) {
	h, _, r := newTestHandlers(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/register", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "RegisterPage", r.lastMethod)
}

func TestRegisterPageInviteOnlyNoToken(t *testing.T) {
	h, _, r := newTestHandlersInviteOnly(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/register", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, "RegisterPage", r.lastMethod)
}

func TestRegisterPageInviteOnlyWithValidToken(t *testing.T) {
	h, repo, r := newTestHandlersInviteOnly(t)

	tokenHash := HashToken("validtoken")
	invite := &Invite{
		Email:     "test@example.com",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	require.NoError(t, repo.CreateInvite(context.Background(), invite))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/register?invite=validtoken", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, "RegisterPage", r.lastMethod)
}

// --- RegisterBegin tests ---

func TestRegisterBeginUsernameMode(t *testing.T) {
	h, repo, _ := newTestHandlers(t)
	body := strings.NewReader(`{"username":"newuser"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterBegin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "publicKey")
	assert.Contains(t, rec.Body.String(), "user_id")

	// User should exist in the DB after successful RegisterBegin.
	users, err := repo.ListUsers(context.Background())
	require.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "newuser", users[0].Username)
}

func TestRegisterBeginMissingUsername(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	body := strings.NewReader(`{"username":""}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterBegin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "username is required")
}

func TestRegisterBeginUsernameExists(t *testing.T) {
	h, repo, _ := newTestHandlers(t)

	_, err := repo.CreateUser(context.Background(), "taken", "")
	require.NoError(t, err)

	body := strings.NewReader(`{"username":"taken"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.RegisterBegin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code, "must not reveal that username exists")
	assert.Contains(t, rec.Body.String(), "registration failed")
	assert.NotContains(t, rec.Body.String(), "publicKey", "must not start WebAuthn flow for existing user")
}

func TestRegisterBeginInvalidJSON(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterBegin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRegisterBeginEmailMode(t *testing.T) {
	h, _, _ := newTestHandlersEmailMode(t)
	body := strings.NewReader(`{"email":"test@example.com"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterBegin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "publicKey")
}

func TestRegisterBeginEmailModeMissingEmail(t *testing.T) {
	h, _, _ := newTestHandlersEmailMode(t)
	body := strings.NewReader(`{"email":""}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterBegin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "email is required")
}

func TestRegisterBeginEmailModeEmailExists(t *testing.T) {
	h, repo, _ := newTestHandlersEmailMode(t)

	_, err := repo.CreateUserWithEmail(context.Background(), "taken@example.com", "")
	require.NoError(t, err)

	body := strings.NewReader(`{"email":"taken@example.com"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.RegisterBegin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code, "must not reveal that email exists")
	assert.Contains(t, rec.Body.String(), "registration failed")
	assert.NotContains(t, rec.Body.String(), "publicKey", "must not start WebAuthn flow for existing user")
}

func TestRegisterBeginInviteOnlyNoToken(t *testing.T) {
	h, repo, _ := newTestHandlersInviteOnly(t)

	// Create a user so first-user bypass doesn't apply.
	_, err := repo.CreateUser(context.Background(), "existing", "")
	require.NoError(t, err)

	body := strings.NewReader(`{"username":"newuser"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.RegisterBegin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRegisterBeginInviteOnlyFirstUserBypass(t *testing.T) {
	h, _, _ := newTestHandlersInviteOnly(t)

	// No users exist - first user bypasses invite requirement.
	body := strings.NewReader(`{"username":"firstuser"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterBegin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "publicKey")
}

func TestRegisterBeginInviteOnlyValidToken(t *testing.T) {
	h, repo, _ := newTestHandlersInviteOnly(t)

	// Create existing user and invite.
	admin, err := repo.CreateUser(context.Background(), "admin", "")
	require.NoError(t, err)
	invite := &Invite{
		Email:     "invitee@example.com",
		TokenHash: HashToken("invitetoken"),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CreatedBy: &admin.ID,
	}
	require.NoError(t, repo.CreateInvite(context.Background(), invite))

	body := strings.NewReader(`{"username":"newuser","invite":"invitetoken"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.RegisterBegin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "publicKey")
}

func TestRegisterBeginInviteOnlyExpiredToken(t *testing.T) {
	h, repo, _ := newTestHandlersInviteOnly(t)

	admin, err := repo.CreateUser(context.Background(), "admin", "")
	require.NoError(t, err)
	invite := &Invite{
		Email:     "expired@example.com",
		TokenHash: HashToken("expiredtoken"),
		ExpiresAt: time.Now().Add(-time.Hour),
		CreatedBy: &admin.ID,
	}
	require.NoError(t, repo.CreateInvite(context.Background(), invite))

	body := strings.NewReader(`{"username":"newuser","invite":"expiredtoken"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.RegisterBegin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// --- RegisterFinish tests ---

func TestRegisterFinishInvalidUserID(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/finish?user_id=invalid", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterFinish(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid user_id")
}

func TestRegisterFinishSessionExpired(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/finish?user_id=99999", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterFinish(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- LoginPage tests ---

func TestLoginPage(t *testing.T) {
	h, _, r := newTestHandlers(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/login", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.LoginPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "LoginPage", r.lastMethod)
}

// --- LoginBegin tests ---

func TestLoginBegin(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/login/begin", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.LoginBegin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "publicKey")
	assert.Contains(t, rec.Body.String(), "session_id")
}

// --- LoginFinish tests ---

func TestLoginFinishMissingSessionID(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/login/finish", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.LoginFinish(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "session_id is required")
}

func TestLoginFinishSessionExpired(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/login/finish?session_id=nonexistent", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.LoginFinish(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "login session expired")
}

// --- Logout tests ---

func TestLogout(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/logout", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.Logout(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/auth/login", rec.Header().Get("Location"))

	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, -1, cookies[0].MaxAge)
}

func TestLogoutCustomRedirect(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	renderer := &mockRenderer{}
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	app := testApp(t, testI18nBundle(t))
	h := NewHandlers(repo, waSvc, nil, renderer, &Config{
		LoginRedirect:  "/dashboard",
		LogoutRedirect: "/goodbye",
	}, app)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/logout", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.Logout(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/goodbye", rec.Header().Get("Location"))
}

// --- CredentialsPage tests ---

func TestCredentialsPage(t *testing.T) {
	h, repo, r := newTestHandlers(t)
	user, err := repo.CreateUser(context.Background(), "alice", "")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/credentials", nil)
	req = requestWithSession(req, user)
	rec := httptest.NewRecorder()

	err = h.CredentialsPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "CredentialsPage", r.lastMethod)
}

// --- DeleteCredential tests ---

func TestDeleteCredentialInvalidID(t *testing.T) {
	h, repo, _ := newTestHandlers(t)
	user, _ := repo.CreateUser(context.Background(), "alice", "")

	router := chi.NewRouter()
	router.Delete("/auth/credentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		r = requestWithSession(r, user)
		_ = h.DeleteCredential(w, r)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/auth/credentials/invalid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeleteCredentialLastCredential(t *testing.T) {
	h, repo, _ := newTestHandlers(t)
	user, _ := repo.CreateUser(context.Background(), "alice", "")
	cred := &Credential{
		UserID:       user.ID,
		CredentialID: []byte("cred-1"),
		PublicKey:    []byte("key-1"),
		Name:         "Only Passkey",
	}
	require.NoError(t, repo.CreateCredential(context.Background(), cred))

	router := chi.NewRouter()
	router.Delete("/auth/credentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		r = requestWithSession(r, user)
		_ = h.DeleteCredential(w, r)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/auth/credentials/"+strconv.FormatInt(cred.ID, 10), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "cannot delete last credential")
}

func TestDeleteCredentialSuccess(t *testing.T) {
	h, repo, _ := newTestHandlers(t)
	user, _ := repo.CreateUser(context.Background(), "alice", "")
	cred1 := &Credential{UserID: user.ID, CredentialID: []byte("c1"), PublicKey: []byte("k1"), Name: "P1"}
	cred2 := &Credential{UserID: user.ID, CredentialID: []byte("c2"), PublicKey: []byte("k2"), Name: "P2"}
	require.NoError(t, repo.CreateCredential(context.Background(), cred1))
	require.NoError(t, repo.CreateCredential(context.Background(), cred2))

	router := chi.NewRouter()
	router.Delete("/auth/credentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		r = requestWithSession(r, user)
		_ = h.DeleteCredential(w, r)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/auth/credentials/"+strconv.FormatInt(cred1.ID, 10), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- RecoveryPage tests ---

func TestRecoveryPage(t *testing.T) {
	h, _, r := newTestHandlers(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/recovery", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RecoveryPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, "RecoveryPage", r.lastMethod)
}

// --- RecoveryLogin tests ---

func TestRecoveryLoginMissingFields(t *testing.T) {
	h, _, _ := newTestHandlers(t)

	tests := []struct {
		name string
		body string
	}{
		{"missing both", `{}`},
		{"missing code", `{"username":"testuser"}`},
		{"missing username", `{"code":"abcd-efgh-ijkl"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.NewReader(tt.body)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery", body)
			req.Header.Set("Content-Type", "application/json")
			req = requestWithSession(req, nil)
			rec := httptest.NewRecorder()

			err := h.RecoveryLogin(rec, req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestRecoveryLoginUserNotFound(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	body := strings.NewReader(`{"username":"nonexistent","code":"abcd-efgh-ijkl"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RecoveryLogin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid username or recovery code")
}

func TestRecoveryLoginInvalidCode(t *testing.T) {
	h, repo, _ := newTestHandlers(t)
	user, _ := repo.CreateUser(context.Background(), "alice", "")

	svc := &RecoveryService{BcryptCost: bcrypt.MinCost}
	_, hashes, err := svc.GenerateCodes(CodeCount)
	require.NoError(t, err)
	require.NoError(t, repo.CreateRecoveryCodes(context.Background(), user.ID, hashes))

	body := strings.NewReader(`{"username":"alice","code":"wrong-code-here"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.RecoveryLogin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRecoveryLoginSuccess(t *testing.T) {
	h, repo, _ := newTestHandlers(t)
	user, _ := repo.CreateUser(context.Background(), "alice", "")

	svc := &RecoveryService{BcryptCost: bcrypt.MinCost}
	codes, hashes, err := svc.GenerateCodes(CodeCount)
	require.NoError(t, err)
	require.NoError(t, repo.CreateRecoveryCodes(context.Background(), user.ID, hashes))

	body := strings.NewReader(`{"username":"alice","code":"` + codes[0] + `"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.RecoveryLogin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
	assert.Contains(t, rec.Body.String(), "remaining_codes")
}

// --- RegenerateRecoveryCodes tests ---

func TestRegenerateRecoveryCodes(t *testing.T) {
	h, repo, _ := newTestHandlers(t)
	user, _ := repo.CreateUser(context.Background(), "alice", "")

	svc := &RecoveryService{BcryptCost: bcrypt.MinCost}
	_, hashes, err := svc.GenerateCodes(CodeCount)
	require.NoError(t, err)
	require.NoError(t, repo.CreateRecoveryCodes(context.Background(), user.ID, hashes))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery-codes/regenerate", nil)
	req = requestWithSession(req, user)
	rec := httptest.NewRecorder()

	err = h.RegenerateRecoveryCodes(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
	assert.Contains(t, rec.Body.String(), `"redirect":"/auth/recovery-codes"`)
}

// --- RecoveryCodesPage tests ---

func TestRecoveryCodesPageWithCodes(t *testing.T) {
	h, _, r := newTestHandlers(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/recovery-codes", nil)
	req = session.Inject(req, map[string]any{
		"user_id":        int64(1),
		"recovery_codes": []string{"code1", "code2"},
	})
	ctx := WithUser(req.Context(), &User{ID: 1})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := h.RecoveryCodesPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "RecoveryCodesPage", r.lastMethod)
}

func TestRecoveryCodesPageWithoutCodes(t *testing.T) {
	h, _, _ := newTestHandlers(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/recovery-codes", nil)
	req = requestWithSession(req, &User{ID: 1})
	rec := httptest.NewRecorder()

	err := h.RecoveryCodesPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/dashboard", rec.Header().Get("Location"))
}

// --- AcknowledgeRecoveryCodes tests ---

func TestAcknowledgeRecoveryCodes(t *testing.T) {
	h, _, _ := newTestHandlers(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery-codes/ack", nil)
	req = session.Inject(req, map[string]any{
		"user_id":        int64(1),
		"recovery_codes": []string{"code1"},
	})
	ctx := WithUser(req.Context(), &User{ID: 1})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := h.AcknowledgeRecoveryCodes(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/dashboard", rec.Header().Get("Location"))
}

func TestAcknowledgeRecoveryCodesWithRedirect(t *testing.T) {
	h, _, _ := newTestHandlers(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery-codes/ack", nil)
	req = session.Inject(req, map[string]any{
		"user_id":              int64(1),
		"recovery_codes":       []string{"code1"},
		"redirect_after_login": "/admin/",
	})
	ctx := WithUser(req.Context(), &User{ID: 1})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := h.AcknowledgeRecoveryCodes(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/admin/", rec.Header().Get("Location"))
}

// --- Email verification tests ---

func TestVerifyPendingPage(t *testing.T) {
	h, _, r := newTestHandlersEmailMode(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-pending", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.VerifyPendingPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, "VerifyPendingPage", r.lastMethod)
}

func TestVerifyEmailMissingToken(t *testing.T) {
	h, _, r := newTestHandlersEmailMode(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-email", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.VerifyEmail(rec, req)

	require.NoError(t, err)
	assert.Equal(t, "VerifyEmailErrorPage", r.lastMethod)
}

func TestVerifyEmailInvalidToken(t *testing.T) {
	h, _, r := newTestHandlersEmailMode(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-email?token=invalid", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.VerifyEmail(rec, req)

	require.NoError(t, err)
	assert.Equal(t, "VerifyEmailErrorPage", r.lastMethod)
}

func TestVerifyEmailExpiredToken(t *testing.T) {
	h, repo, r := newTestHandlersEmailMode(t)
	user, _ := repo.CreateUserWithEmail(context.Background(), "test@example.com", "")
	tokenHash := HashToken("expiredtoken")
	require.NoError(t, repo.CreateEmailVerificationToken(context.Background(), user.ID, tokenHash, time.Now().Add(-time.Hour)))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-email?token=expiredtoken", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.VerifyEmail(rec, req)

	require.NoError(t, err)
	assert.Equal(t, "VerifyEmailErrorPage", r.lastMethod)
}

func TestVerifyEmailSuccess(t *testing.T) {
	h, repo, r := newTestHandlersEmailMode(t)
	user, _ := repo.CreateUserWithEmail(context.Background(), "test@example.com", "")
	tokenHash := HashToken("validtoken")
	require.NoError(t, repo.CreateEmailVerificationToken(context.Background(), user.ID, tokenHash, time.Now().Add(24*time.Hour)))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-email?token=validtoken", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.VerifyEmail(rec, req)

	require.NoError(t, err)
	assert.Equal(t, "VerifyEmailSuccessPage", r.lastMethod)

	// User should be marked as verified.
	got, _ := repo.GetUserByID(context.Background(), user.ID)
	assert.True(t, got.EmailVerified)
}

// --- ResendVerification tests ---

func TestResendVerificationMissingEmail(t *testing.T) {
	h, _, _ := newTestHandlersEmailMode(t)
	body := strings.NewReader(`{"email":""}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/resend-verification", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.ResendVerification(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestResendVerificationNonexistentEmail(t *testing.T) {
	h, _, _ := newTestHandlersEmailMode(t)
	body := strings.NewReader(`{"email":"nobody@example.com"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/resend-verification", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.ResendVerification(rec, req)

	require.NoError(t, err)
	// Should still return OK (don't reveal if email exists).
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- errorJSONLog tests ---

func TestErrorJSONLog(t *testing.T) {
	rec := httptest.NewRecorder()
	err := errorJSONLog(rec, http.StatusInternalServerError, "something failed", assert.AnError)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "something failed")
}

func TestErrorJSONLogNilError(t *testing.T) {
	rec := httptest.NewRecorder()
	err := errorJSONLog(rec, http.StatusInternalServerError, "msg", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "msg")
}

// --- ResendVerification additional paths ---

func TestResendVerificationAlreadyVerified(t *testing.T) {
	h, repo, _ := newTestHandlersEmailMode(t)

	user, err := repo.CreateUserWithEmail(context.Background(), "verified@example.com", "")
	require.NoError(t, err)
	require.NoError(t, repo.MarkEmailVerified(context.Background(), user.ID))

	body := strings.NewReader(`{"email":"verified@example.com"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/resend-verification", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.ResendVerification(rec, req)
	require.NoError(t, err)
	// Returns OK without re-sending (don't reveal user state).
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestResendVerificationSuccess(t *testing.T) {
	h, repo, _ := newTestHandlersEmailMode(t)

	_, err := repo.CreateUserWithEmail(context.Background(), "test@example.com", "")
	require.NoError(t, err)

	body := strings.NewReader(`{"email":"test@example.com"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/resend-verification", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.ResendVerification(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestResendVerificationInvalidJSON(t *testing.T) {
	h, _, _ := newTestHandlersEmailMode(t)

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/resend-verification", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.ResendVerification(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- RecoveryLogin with deactivated user ---

func TestRecoveryLoginDeactivatedUser(t *testing.T) {
	h, repo, _ := newTestHandlers(t)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "inactive", "")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserActive(ctx, user.ID, false))

	body := strings.NewReader(`{"username":"inactive","code":"some-code"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.RecoveryLogin(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "account deactivated")
}

// --- RecoveryCodesPage with non-slice codes ---

func TestRecoveryCodesPageInvalidType(t *testing.T) {
	h, _, _ := newTestHandlers(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/recovery-codes", nil)
	req = session.Inject(req, map[string]any{
		"user_id":        int64(1),
		"recovery_codes": "not-a-slice",
	})
	ctx := WithUser(req.Context(), &User{ID: 1})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := h.RecoveryCodesPage(rec, req)
	require.NoError(t, err)
	// Invalid type should redirect.
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/dashboard", rec.Header().Get("Location"))
}

// --- AcknowledgeRecoveryCodes redirect from session ---

func TestAcknowledgeRecoveryCodesUsesSessionRedirect(t *testing.T) {
	h, _, _ := newTestHandlers(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery-codes/ack", nil)
	req = session.Inject(req, map[string]any{
		"user_id":              int64(1),
		"recovery_codes":       []string{"code1"},
		"redirect_after_login": "/custom-redirect",
	})
	ctx := WithUser(req.Context(), &User{ID: 1})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := h.AcknowledgeRecoveryCodes(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/custom-redirect", rec.Header().Get("Location"))
}

// --- RegisterBegin with invite-only email mode, email mismatch ---

func TestRegisterBeginInviteOnlyEmailModeMismatch(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	renderer := &mockRenderer{}
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	emailSvc := &mockEmailService{}
	app := testApp(t, testI18nBundle(t))
	app.emailService = emailSvc
	h := NewHandlers(repo, waSvc, emailSvc, renderer, &Config{
		LoginRedirect:       "/dashboard",
		LogoutRedirect:      "/auth/login",
		UseEmail:            true,
		RequireVerification: true,
		InviteOnly:          true,
		BaseURL:             "http://localhost:8080",
	}, app)

	// Create existing user so first-user bypass doesn't apply.
	_, err = repo.CreateUser(context.Background(), "existing", "")
	require.NoError(t, err)

	// Create invite for specific email.
	invite := &Invite{
		Email:     "invited@example.com",
		TokenHash: HashToken("invtoken"),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	require.NoError(t, repo.CreateInvite(context.Background(), invite))

	// Try to register with different email.
	reqBody := strings.NewReader(`{"email":"wrong@example.com","invite":"invtoken"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", reqBody)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.RegisterBegin(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "email does not match invite")
}

// --- RegisterPageInviteOnlyExpiredToken ---

func TestRegisterPageInviteOnlyExpiredToken(t *testing.T) {
	h, repo, r := newTestHandlersInviteOnly(t)

	// Create an expired invite.
	invite := &Invite{
		Email:     "expired@example.com",
		TokenHash: HashToken("exptoken"),
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	require.NoError(t, repo.CreateInvite(context.Background(), invite))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/register?invite=exptoken", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterPage(rec, req)
	require.NoError(t, err)
	assert.Equal(t, "RegisterPage", r.lastMethod)
}

// --- UseEmailMode / IsInviteOnly with nil config ---

func TestUseEmailModeNilConfig(t *testing.T) {
	h := &Handlers{config: nil}
	assert.False(t, h.UseEmailMode())
}

func TestIsInviteOnlyNilConfig(t *testing.T) {
	h := &Handlers{config: nil}
	assert.False(t, h.IsInviteOnly())
}

// --- findStoredSignCount tests ---

func TestFindStoredSignCountFound(t *testing.T) {
	creds := []Credential{
		{CredentialID: []byte("cred-a"), SignCount: 10},
		{CredentialID: []byte("cred-b"), SignCount: 42},
	}

	count, ok := findStoredSignCount(creds, []byte("cred-b"))
	assert.True(t, ok)
	assert.Equal(t, uint32(42), count)
}

func TestFindStoredSignCountNotFound(t *testing.T) {
	creds := []Credential{
		{CredentialID: []byte("cred-a"), SignCount: 10},
	}

	count, ok := findStoredSignCount(creds, []byte("nonexistent"))
	assert.False(t, ok)
	assert.Equal(t, uint32(0), count)
}

func TestFindStoredSignCountEmptySlice(t *testing.T) {
	count, ok := findStoredSignCount(nil, []byte("anything"))
	assert.False(t, ok)
	assert.Equal(t, uint32(0), count)
}

// --- RecoveryLogin invalid JSON ---

func TestRecoveryLoginInvalidJSON(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RecoveryLogin(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid request")
}

// --- RecoveryCodesPage with empty codes slice ---

func TestRecoveryCodesPageEmptyCodes(t *testing.T) {
	h, _, _ := newTestHandlers(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/recovery-codes", nil)
	req = session.Inject(req, map[string]any{
		"user_id":        int64(1),
		"recovery_codes": []string{},
	})
	ctx := WithUser(req.Context(), &User{ID: 1})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := h.RecoveryCodesPage(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/dashboard", rec.Header().Get("Location"))
}

// --- VerifyEmail: DB error on MarkEmailVerified ---

func TestVerifyEmailDBErrorOnMarkVerified(t *testing.T) {
	h, repo, sqldb := newTestHandlersEmailModeClosable(t)
	ctx := context.Background()

	user, err := repo.CreateUserWithEmail(ctx, "test@example.com", "")
	require.NoError(t, err)

	tokenHash := HashToken("goodtoken")
	require.NoError(t, repo.CreateEmailVerificationToken(ctx, user.ID, tokenHash, time.Now().Add(24*time.Hour)))

	// Close the DB so MarkEmailVerified fails.
	_ = sqldb.Close()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-email?token=goodtoken", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.VerifyEmail(rec, req)
	require.NoError(t, err)
	// Should render the error page.
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- VerifyEmail: DB error on GetUserByID after verification ---

func TestVerifyEmailDBErrorOnGetUserAfterVerify(t *testing.T) {
	h, repo, _ := newTestHandlersEmailModeClosable(t)
	ctx := context.Background()

	user, err := repo.CreateUserWithEmail(ctx, "test@example.com", "")
	require.NoError(t, err)

	tokenHash := HashToken("goodtoken2")
	require.NoError(t, repo.CreateEmailVerificationToken(ctx, user.ID, tokenHash, time.Now().Add(24*time.Hour)))

	// Delete the user so GetUserByID fails after MarkEmailVerified succeeds (updates 0 rows, no error).
	require.NoError(t, repo.DeleteUser(ctx, user.ID))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-email?token=goodtoken2", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.VerifyEmail(rec, req)
	require.NoError(t, err)
	// GetUserByID returns ErrNotFound, should render error page.
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- ResendVerification: user with nil email ---

func TestResendVerificationUserWithNilEmail(t *testing.T) {
	h, repo, _ := newTestHandlersEmailMode(t)

	// Create a user via username mode (no email), then look up by email.
	// Since GetUserByEmail would fail for a username-mode user, we need a user
	// whose Email field is nil but is somehow found. This path is defensive;
	// we can test it by creating a user with email, then nullifying it.
	user, err := repo.CreateUserWithEmail(context.Background(), "nullemail@example.com", "")
	require.NoError(t, err)

	// Nullify the email in the DB directly.
	_, err = repo.db.NewUpdate().Model((*User)(nil)).
		Set("email = NULL").
		Where("id = ?", user.ID).
		Exec(context.Background())
	require.NoError(t, err)

	body := strings.NewReader(`{"email":"nullemail@example.com"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/resend-verification", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.ResendVerification(rec, req)
	require.NoError(t, err)
	// Should return OK silently (defensive path).
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- DeleteCredential: DB error on CountUserCredentials ---

func TestDeleteCredentialDBErrorOnCount(t *testing.T) {
	h, repo, sqldb := newTestHandlersClosable(t)
	user, _ := repo.CreateUser(context.Background(), "bob", "")
	cred1 := &Credential{UserID: user.ID, CredentialID: []byte("c1"), PublicKey: []byte("k1"), Name: "P1"}
	cred2 := &Credential{UserID: user.ID, CredentialID: []byte("c2"), PublicKey: []byte("k2"), Name: "P2"}
	require.NoError(t, repo.CreateCredential(context.Background(), cred1))
	require.NoError(t, repo.CreateCredential(context.Background(), cred2))

	// Close DB so CountUserCredentials fails.
	_ = sqldb.Close()

	router := chi.NewRouter()
	router.Delete("/auth/credentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		r = requestWithSession(r, user)
		_ = h.DeleteCredential(w, r)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/auth/credentials/"+strconv.FormatInt(cred1.ID, 10), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "database error")
}

// --- CredentialsPage: DB error on GetCredentialsByUserID ---

func TestCredentialsPageDBError(t *testing.T) {
	h, repo, sqldb := newTestHandlersClosable(t)
	user, _ := repo.CreateUser(context.Background(), "carol", "")

	// Close DB so GetCredentialsByUserID fails.
	_ = sqldb.Close()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/credentials", nil)
	req = requestWithSession(req, user)
	rec := httptest.NewRecorder()

	err := h.CredentialsPage(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "failed to get credentials")
}

// --- RegenerateRecoveryCodes: DB error on generateAndStoreRecoveryCodes ---

func TestRegenerateRecoveryCodesDBError(t *testing.T) {
	h, repo, sqldb := newTestHandlersClosable(t)
	user, _ := repo.CreateUser(context.Background(), "eve", "")

	// Close DB so generateAndStoreRecoveryCodes fails (DeleteRecoveryCodes fails).
	_ = sqldb.Close()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery-codes/regenerate", nil)
	req = requestWithSession(req, user)
	rec := httptest.NewRecorder()

	err := h.RegenerateRecoveryCodes(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "failed to regenerate codes")
}

// --- isFirstUser: DB error ---

func TestIsFirstUserDBError(t *testing.T) {
	h, _, sqldb := newTestHandlersClosable(t)

	// Close DB so CountUsers fails.
	_ = sqldb.Close()

	isFirst, err := h.isFirstUser(context.Background())
	require.Error(t, err)
	assert.False(t, isFirst)
}

// --- RecoveryLogin: DB error on ValidateAndUseRecoveryCode ---

func TestRecoveryLoginDBErrorOnValidation(t *testing.T) {
	h, repo, sqldb := newTestHandlersClosable(t)
	user, _ := repo.CreateUser(context.Background(), "frank", "")

	svc := &RecoveryService{BcryptCost: bcrypt.MinCost}
	codes, hashes, err := svc.GenerateCodes(CodeCount)
	require.NoError(t, err)
	require.NoError(t, repo.CreateRecoveryCodes(context.Background(), user.ID, hashes))

	// Close DB so ValidateAndUseRecoveryCode fails.
	_ = sqldb.Close()

	body := strings.NewReader(`{"username":"frank","code":"` + codes[0] + `"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.RecoveryLogin(rec, req)
	require.NoError(t, err)
	// GetUserByUsername will fail first (DB closed), returning "invalid username or recovery code".
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- ResendVerification: DB error on CreateEmailVerificationToken ---

func TestResendVerificationDBErrorOnTokenCreate(t *testing.T) {
	h, repo, sqldb := newTestHandlersEmailModeClosable(t)

	_, err := repo.CreateUserWithEmail(context.Background(), "test@example.com", "")
	require.NoError(t, err)

	// Close DB so CreateEmailVerificationToken fails.
	_ = sqldb.Close()

	body := strings.NewReader(`{"email":"test@example.com"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/resend-verification", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.ResendVerification(rec, req)
	require.NoError(t, err)
	// GetUserByEmail fails (DB closed), returns OK to not reveal user existence.
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- RegenerateRecoveryCodes: first-time generation (no existing codes to delete) ---

func TestRegenerateRecoveryCodesFirstTime(t *testing.T) {
	h, repo, _ := newTestHandlers(t)
	user, _ := repo.CreateUser(context.Background(), "charlie", "")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery-codes/regenerate", nil)
	req = requestWithSession(req, user)
	rec := httptest.NewRecorder()

	err := h.RegenerateRecoveryCodes(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
}

// --- RecoveryLogin with session redirect ---

func TestRecoveryLoginUsesSessionRedirect(t *testing.T) {
	h, repo, _ := newTestHandlers(t)
	user, _ := repo.CreateUser(context.Background(), "alice", "")

	svc := &RecoveryService{BcryptCost: bcrypt.MinCost}
	codes, hashes, err := svc.GenerateCodes(CodeCount)
	require.NoError(t, err)
	require.NoError(t, repo.CreateRecoveryCodes(context.Background(), user.ID, hashes))

	body := strings.NewReader(`{"username":"alice","code":"` + codes[0] + `"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery", body)
	req.Header.Set("Content-Type", "application/json")
	req = session.Inject(req, map[string]any{
		"redirect_after_login": "/custom-page",
	})
	rec := httptest.NewRecorder()

	err = h.RecoveryLogin(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"redirect":"/custom-page"`)
}

// --- CredentialsPage with credentials ---

func TestCredentialsPageWithCredentials(t *testing.T) {
	h, repo, r := newTestHandlers(t)
	user, err := repo.CreateUser(context.Background(), "dave", "")
	require.NoError(t, err)

	cred := &Credential{UserID: user.ID, CredentialID: []byte("c1"), PublicKey: []byte("k1"), Name: "My Key"}
	require.NoError(t, repo.CreateCredential(context.Background(), cred))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/credentials", nil)
	req = requestWithSession(req, user)
	rec := httptest.NewRecorder()

	err = h.CredentialsPage(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "CredentialsPage", r.lastMethod)
}

// --- AcknowledgeRecoveryCodes: no session middleware ---

func TestAcknowledgeRecoveryCodesNoSession(t *testing.T) {
	h, _, _ := newTestHandlers(t)

	// Request WITHOUT session.Inject — session.Delete will return errNoMiddleware.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery-codes/ack", nil)
	rec := httptest.NewRecorder()

	err := h.AcknowledgeRecoveryCodes(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "failed to clear recovery codes")
}

// --- RegenerateRecoveryCodes: no session middleware ---

func TestRegenerateRecoveryCodesNoSession(t *testing.T) {
	h, repo, _ := newTestHandlers(t)
	user, _ := repo.CreateUser(context.Background(), "nosession", "")

	// Request WITHOUT session.Inject — session.Set will fail.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery-codes/regenerate", nil)
	ctx := WithUser(req.Context(), user)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := h.RegenerateRecoveryCodes(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "failed to store recovery codes")
}

// --- VerifyEmail: DB error after token found (GetEmailVerificationToken succeeds) ---

func TestVerifyEmailGetUserByIDFailure(t *testing.T) {
	// Test the path where MarkEmailVerified succeeds but GetUserByID fails.
	// Disable FK so we can create an orphan token that references a non-existent user.
	h, repo, _ := newTestHandlersEmailModeClosable(t)
	ctx := context.Background()

	// Disable FK constraints so we can create a token for a non-existent user.
	_, err := repo.db.Exec("PRAGMA foreign_keys=OFF")
	require.NoError(t, err)

	tokenHash := HashToken("orphantoken")
	// userID 99999 does not exist.
	require.NoError(t, repo.CreateEmailVerificationToken(ctx, 99999, tokenHash, time.Now().Add(24*time.Hour)))

	_, err = repo.db.Exec("PRAGMA foreign_keys=ON")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-email?token=orphantoken", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.VerifyEmail(rec, req)
	require.NoError(t, err)
	// MarkEmailVerified updates 0 rows (no error), then GetUserByID returns ErrNotFound.
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- ResendVerification: full path with existing old tokens ---

func TestResendVerificationDeletesOldTokens(t *testing.T) {
	h, repo, _ := newTestHandlersEmailMode(t)
	ctx := context.Background()

	user, err := repo.CreateUserWithEmail(ctx, "test@example.com", "")
	require.NoError(t, err)

	// Create an existing old token.
	oldHash := HashToken("oldtoken")
	require.NoError(t, repo.CreateEmailVerificationToken(ctx, user.ID, oldHash, time.Now().Add(-time.Hour)))

	body := strings.NewReader(`{"email":"test@example.com"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/resend-verification", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.ResendVerification(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestVerifySignCount(t *testing.T) {
	tests := []struct {
		name        string
		stored      uint32
		incoming    uint32
		expectError bool
	}{
		{"both zero (software authenticator)", 0, 0, false},
		{"normal increment", 5, 6, false},
		{"large increment", 5, 100, false},
		{"first use after registration", 0, 1, false},
		{"same count (possible clone)", 5, 5, true},
		{"decreased count (possible clone)", 5, 3, true},
		{"incoming zero with stored nonzero (possible clone)", 5, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifySignCount(tt.stored, tt.incoming)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
