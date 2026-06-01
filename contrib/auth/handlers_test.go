package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/oliverandrich/burrow/i18n"
	"github.com/oliverandrich/den"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// testUserWithID returns a User with a fixed test ID.
func testUserWithID() *User[EmptyProfile] {
	u := &User[EmptyProfile]{Username: "testuser", IsActive: true}
	u.ID = "test-user-1"
	return u
}

// --- Mocks ---

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
	bundle, err := i18n.NewTestBundle(translationFS)
	require.NoError(t, err)
	return bundle
}

func testApp(t *testing.T, bundle *i18n.Bundle) *App[EmptyProfile] {
	t.Helper()
	return &App[EmptyProfile]{withLocale: bundle.WithLocale}
}

func setupTestApp(t *testing.T) (*App[EmptyProfile], *Repository[EmptyProfile]) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	a := testApp(t, testI18nBundle(t))
	a.repo = repo
	a.webauthn = waSvc
	a.config = &Config{
		LoginRedirect:  "/dashboard",
		LogoutRedirect: "/auth/login",
	}
	a.recovery = NewRecoveryService()
	a.recovery.BcryptCost = bcrypt.MinCost
	return a, repo
}

func setupTestAppEmailMode(t *testing.T) (*App[EmptyProfile], *Repository[EmptyProfile]) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	emailSvc := &mockEmailService{}
	a := testApp(t, testI18nBundle(t))
	a.repo = repo
	a.webauthn = waSvc
	a.emailService = emailSvc
	a.config = &Config{
		LoginRedirect:       "/dashboard",
		LogoutRedirect:      "/auth/login",
		UseEmail:            true,
		RequireVerification: true,
		BaseURL:             "http://localhost:8080",
	}
	a.recovery = NewRecoveryService()
	a.recovery.BcryptCost = bcrypt.MinCost
	return a, repo
}

func setupTestAppInviteOnly(t *testing.T) (*App[EmptyProfile], *Repository[EmptyProfile]) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	a := testApp(t, testI18nBundle(t))
	a.repo = repo
	a.webauthn = waSvc
	a.config = &Config{
		LoginRedirect:  "/dashboard",
		LogoutRedirect: "/auth/login",
		InviteOnly:     true,
	}
	a.recovery = NewRecoveryService()
	a.recovery.BcryptCost = bcrypt.MinCost
	return a, repo
}

// requestWithSession creates a request with session state injected, optionally with a user.
// It also installs a real template executor in context so page-rendering handlers
// (loginPage, registerPage, …) produce HTML instead of falling through to burrow.Render.
func requestWithSession(req *http.Request, user *User[EmptyProfile]) *http.Request {
	req = session.Inject(req, map[string]any{})
	ctx := burrow.WithTemplateExecutor(req.Context(), rendererTestExecutor())
	if user != nil {
		ctx = WithUser(ctx, user)
	}
	return req.WithContext(ctx)
}

// openTestDBClosable opens a test DB that can be closed to trigger database errors in handlers.
// It returns the *den.DB (closable via db.Close()).
func openTestDBClosable(t *testing.T) *den.DB {
	t.Helper()
	db := openTestDB(t)
	return db
}

// setupTestAppClosable creates an App with a DB that can be closed to trigger errors.
func setupTestAppClosable(t *testing.T) (*App[EmptyProfile], *Repository[EmptyProfile], *den.DB) {
	t.Helper()
	db := openTestDBClosable(t)
	repo := NewRepository[EmptyProfile](db)
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	a := testApp(t, testI18nBundle(t))
	a.repo = repo
	a.webauthn = waSvc
	a.config = &Config{
		LoginRedirect:  "/dashboard",
		LogoutRedirect: "/auth/login",
	}
	a.recovery = NewRecoveryService()
	a.recovery.BcryptCost = bcrypt.MinCost
	return a, repo, db
}

// setupTestAppEmailModeClosable creates an email-mode App with a closable DB.
func setupTestAppEmailModeClosable(t *testing.T) (*App[EmptyProfile], *Repository[EmptyProfile], *den.DB) {
	t.Helper()
	db := openTestDBClosable(t)
	repo := NewRepository[EmptyProfile](db)
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	emailSvc := &mockEmailService{}
	a := testApp(t, testI18nBundle(t))
	a.repo = repo
	a.webauthn = waSvc
	a.emailService = emailSvc
	a.config = &Config{
		LoginRedirect:       "/dashboard",
		LogoutRedirect:      "/auth/login",
		UseEmail:            true,
		RequireVerification: true,
		BaseURL:             "http://localhost:8080",
	}
	a.recovery = NewRecoveryService()
	a.recovery.BcryptCost = bcrypt.MinCost
	return a, repo, db
}

// --- Handler creation tests ---

func TestAppSetup(t *testing.T) {
	h, _ := setupTestApp(t)
	assert.NotNil(t, h)
	assert.False(t, h.UseEmailMode())
	assert.False(t, h.IsInviteOnly())
}

func TestAppSetupEmailMode(t *testing.T) {
	h, _ := setupTestAppEmailMode(t)
	assert.True(t, h.UseEmailMode())
}

func TestAppSetupInviteOnly(t *testing.T) {
	h, _ := setupTestAppInviteOnly(t)
	assert.True(t, h.IsInviteOnly())
}

// --- RegisterPage tests ---

func TestRegisterPage(t *testing.T) {
	h, _ := setupTestApp(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/register", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "register-username-label")
}

func TestRegisterPageInviteOnlyNoToken(t *testing.T) {
	h, _ := setupTestAppInviteOnly(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/register", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "register-username-label")
}

func TestRegisterPageInviteOnlyWithValidToken(t *testing.T) {
	h, repo := setupTestAppInviteOnly(t)

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
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "register-username-label")
}

// --- RegisterBegin tests ---

func TestRegisterBeginUsernameMode(t *testing.T) {
	h, repo := setupTestApp(t)
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

func TestRegisterBeginAppliesDefaultRole(t *testing.T) {
	h, repo := setupTestApp(t)
	ctx := context.Background()

	// Seed an existing admin so the first-user promotion doesn't kick
	// in and shadow the default-role assignment we're verifying.
	admin, err := repo.CreateUser(ctx, "existing-admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, admin.ID, RoleAdmin))

	h.defaultRole = RoleStaff

	body := strings.NewReader(`{"username":"newcomer"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.RegisterBegin(rec, req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	got, err := repo.GetUserByUsername(ctx, "newcomer")
	require.NoError(t, err)
	assert.Equal(t, RoleStaff, got.Role,
		"WithDefaultRole(RoleStaff) should bump new users from RoleUser to RoleStaff")
}

func TestRegisterBeginAdminPromotionWinsOverDefaultRole(t *testing.T) {
	h, repo := setupTestApp(t)
	ctx := context.Background()

	// No prior users → the registrant is the first → admin promotion
	// fires before our default-role check sees user.Role == RoleUser.
	h.defaultRole = RoleStaff

	body := strings.NewReader(`{"username":"firstever"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterBegin(rec, req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	got, err := repo.GetUserByUsername(ctx, "firstever")
	require.NoError(t, err)
	assert.Equal(t, RoleAdmin, got.Role,
		"first user must become admin even when WithDefaultRole is set")
}

func TestRegisterBeginDefaultRoleUnsetPreservesRoleUser(t *testing.T) {
	h, repo := setupTestApp(t)
	ctx := context.Background()

	// Seed an existing admin so the first-user promotion is out of the
	// picture; the only thing being tested is "no option → no change".
	admin, err := repo.CreateUser(ctx, "existing-admin")
	require.NoError(t, err)
	require.NoError(t, repo.SetUserRole(ctx, admin.ID, RoleAdmin))

	// Sanity: option is unset on the test app.
	require.Empty(t, h.defaultRole)

	body := strings.NewReader(`{"username":"plainuser"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.RegisterBegin(rec, req)
	require.NoError(t, err)

	got, err := repo.GetUserByUsername(ctx, "plainuser")
	require.NoError(t, err)
	assert.Equal(t, RoleUser, got.Role)
}

func TestRegisterBeginMissingUsername(t *testing.T) {
	h, _ := setupTestApp(t)
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
	h, repo := setupTestApp(t)

	_, err := repo.CreateUser(context.Background(), "taken")
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

// postRegisterBegin issues a register/begin request with the given JSON
// body and returns the recorder. Shared by the validator table tests.
func postRegisterBegin(t *testing.T, h *App[EmptyProfile], jsonBody string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", strings.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()
	require.NoError(t, h.RegisterBegin(rec, req))
	return rec
}

// TestRegisterBeginValidator covers the username- and email-mode
// registration validators: a non-nil error rejects with 400 and the
// message reaches the user without creating a user, and a passing
// validator is invoked and lets registration proceed.
func TestRegisterBeginValidator(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T) (*App[EmptyProfile], *Repository[EmptyProfile])
		install func(h *App[EmptyProfile], fn func(context.Context, string) error)
		body    string
		stored  string
	}{
		{
			name:    "username mode",
			setup:   setupTestApp,
			install: func(h *App[EmptyProfile], fn func(context.Context, string) error) { h.usernameValidator = fn },
			body:    `{"username":"candidate"}`,
			stored:  "candidate",
		},
		{
			name:    "email mode",
			setup:   setupTestAppEmailMode,
			install: func(h *App[EmptyProfile], fn func(context.Context, string) error) { h.emailValidator = fn },
			body:    `{"email":"candidate@example.com"}`,
			stored:  "candidate@example.com",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name+" rejects", func(t *testing.T) {
			h, repo := tc.setup(t)
			tc.install(h, func(context.Context, string) error { return errors.New("value is not allowed") })

			rec := postRegisterBegin(t, h, tc.body)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), "value is not allowed", "validator error message must reach the user")
			assert.NotContains(t, rec.Body.String(), "publicKey", "must not start WebAuthn flow for rejected value")

			users, err := repo.ListUsers(context.Background())
			require.NoError(t, err)
			assert.Empty(t, users, "rejected value must not create a user")
		})

		t.Run(tc.name+" passes", func(t *testing.T) {
			h, repo := tc.setup(t)
			called := false
			tc.install(h, func(context.Context, string) error { called = true; return nil })

			rec := postRegisterBegin(t, h, tc.body)

			assert.True(t, called, "validator must be invoked")
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), "publicKey")

			users, err := repo.ListUsers(context.Background())
			require.NoError(t, err)
			require.Len(t, users, 1)
			assert.Equal(t, tc.stored, users[0].Username)
		})
	}
}

func TestRegisterBeginNilUsernameValidatorUnchanged(t *testing.T) {
	h, repo := setupTestApp(t)
	require.Nil(t, h.usernameValidator, "default app must have no username validator")

	rec := postRegisterBegin(t, h, `{"username":"newuser"}`)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "publicKey")

	users, err := repo.ListUsers(context.Background())
	require.NoError(t, err)
	assert.Len(t, users, 1)
}

func TestRegisterBeginInvalidJSON(t *testing.T) {
	h, _ := setupTestApp(t)
	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterBegin(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRegisterBeginUsernameValidatorIgnoredInEmailMode(t *testing.T) {
	h, _ := setupTestAppEmailMode(t)
	h.usernameValidator = func(_ context.Context, _ string) error {
		return errors.New("must not run in email mode")
	}

	rec := postRegisterBegin(t, h, `{"email":"someone@example.com"}`)

	assert.Equal(t, http.StatusOK, rec.Code, "username validator must not gate email-mode registration")
	assert.Contains(t, rec.Body.String(), "publicKey")
}

func TestRegisterBeginEmailMode(t *testing.T) {
	h, _ := setupTestAppEmailMode(t)
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
	h, _ := setupTestAppEmailMode(t)
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
	h, repo := setupTestAppEmailMode(t)

	_, err := repo.CreateUserWithEmail(context.Background(), "taken@example.com")
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
	h, repo := setupTestAppInviteOnly(t)

	// Create a user so first-user bypass doesn't apply.
	_, err := repo.CreateUser(context.Background(), "existing")
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
	h, _ := setupTestAppInviteOnly(t)

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
	h, repo := setupTestAppInviteOnly(t)

	// Create existing user and invite.
	admin, err := repo.CreateUser(context.Background(), "admin")
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
	h, repo := setupTestAppInviteOnly(t)

	admin, err := repo.CreateUser(context.Background(), "admin")
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
	h, _ := setupTestApp(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/finish?user_id=invalid", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterFinish(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	// With string IDs, "invalid" is a valid ID format but has no registration session.
	assert.Contains(t, rec.Body.String(), "registration session expired")
}

func TestRegisterFinishSessionExpired(t *testing.T) {
	h, _ := setupTestApp(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/register/finish?user_id=99999", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RegisterFinish(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- LoginPage tests ---

func TestLoginPage(t *testing.T) {
	h, _ := setupTestApp(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/login", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.LoginPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "login-button")
}

// --- LoginBegin tests ---

func TestLoginBegin(t *testing.T) {
	h, _ := setupTestApp(t)
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
	h, _ := setupTestApp(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/login/finish", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.LoginFinish(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "session_id is required")
}

func TestLoginFinishSessionExpired(t *testing.T) {
	h, _ := setupTestApp(t)
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
	h, _ := setupTestApp(t)
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
	repo := NewRepository[EmptyProfile](db)
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	h := testApp(t, testI18nBundle(t))
	h.repo = repo
	h.webauthn = waSvc
	h.config = &Config{
		LoginRedirect:  "/dashboard",
		LogoutRedirect: "/goodbye",
	}

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
	h, repo := setupTestApp(t)
	user, err := repo.CreateUser(context.Background(), "alice")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/credentials", nil)
	req = requestWithSession(req, user)
	rec := httptest.NewRecorder()

	err = h.CredentialsPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "credentials-title")
}

// --- DeleteCredential tests ---

func TestDeleteCredentialInvalidID(t *testing.T) {
	h, repo := setupTestApp(t)
	user, _ := repo.CreateUser(context.Background(), "alice")

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
	h, repo := setupTestApp(t)
	user, _ := repo.CreateUser(context.Background(), "alice")
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

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/auth/credentials/"+cred.ID, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "cannot delete last credential")
}

func TestDeleteCredentialSuccess(t *testing.T) {
	h, repo := setupTestApp(t)
	user, _ := repo.CreateUser(context.Background(), "alice")
	cred1 := &Credential{UserID: user.ID, CredentialID: []byte("c1"), PublicKey: []byte("k1"), Name: "P1"}
	cred2 := &Credential{UserID: user.ID, CredentialID: []byte("c2"), PublicKey: []byte("k2"), Name: "P2"}
	require.NoError(t, repo.CreateCredential(context.Background(), cred1))
	require.NoError(t, repo.CreateCredential(context.Background(), cred2))

	router := chi.NewRouter()
	router.Delete("/auth/credentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		r = requestWithSession(r, user)
		_ = h.DeleteCredential(w, r)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/auth/credentials/"+cred1.ID, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- RecoveryPage tests ---

func TestRecoveryPage(t *testing.T) {
	h, _ := setupTestApp(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/recovery", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.RecoveryPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "recovery-description")
}

// --- RecoveryLogin tests ---

func TestRecoveryLoginMissingFields(t *testing.T) {
	h, _ := setupTestApp(t)

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
	h, _ := setupTestApp(t)
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
	h, repo := setupTestApp(t)
	user, _ := repo.CreateUser(context.Background(), "alice")

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
	h, repo := setupTestApp(t)
	user, _ := repo.CreateUser(context.Background(), "alice")

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

func TestRecoveryLoginLastCodeTriggersRegenerate(t *testing.T) {
	h, repo := setupTestApp(t)
	user, _ := repo.CreateUser(context.Background(), "alice")

	svc := &RecoveryService{BcryptCost: bcrypt.MinCost}
	codes, hashes, err := svc.GenerateCodes(CodeCount)
	require.NoError(t, err)
	require.NoError(t, repo.CreateRecoveryCodes(context.Background(), user.ID, hashes[:1]))
	consumedCode := codes[0]

	body := strings.NewReader(`{"username":"alice","code":"` + consumedCode + `"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.RecoveryLogin(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"redirect":"/auth/recovery-codes"`)
	assert.NotContains(t, rec.Body.String(), "remaining_codes")

	values := session.GetValues(req)
	require.NotNil(t, values)
	assert.Equal(t, user.ID, values["user_id"])
	storedCodes, ok := values["recovery_codes"].([]string)
	require.True(t, ok, "expected recovery_codes in session")
	assert.Len(t, storedCodes, CodeCount)
	assert.NotContains(t, storedCodes, consumedCode, "regenerated codes must differ from the consumed one")
	assert.Equal(t, h.config.LoginRedirect, values["redirect_after_login"])

	freshCodes, err := repo.GetUnusedRecoveryCodes(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Len(t, freshCodes, CodeCount)
}

func TestRecoveryLoginPenultimateCodeKeepsHappyPath(t *testing.T) {
	h, repo := setupTestApp(t)
	user, _ := repo.CreateUser(context.Background(), "alice")

	svc := &RecoveryService{BcryptCost: bcrypt.MinCost}
	codes, hashes, err := svc.GenerateCodes(CodeCount)
	require.NoError(t, err)
	require.NoError(t, repo.CreateRecoveryCodes(context.Background(), user.ID, hashes[:2]))

	body := strings.NewReader(`{"username":"alice","code":"` + codes[0] + `"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery", body)
	req.Header.Set("Content-Type", "application/json")
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.RecoveryLogin(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"remaining_codes":1`)
	assert.Contains(t, rec.Body.String(), `"redirect":"`+h.config.LoginRedirect+`"`)

	values := session.GetValues(req)
	require.NotNil(t, values)
	assert.Equal(t, user.ID, values["user_id"])
	_, hasCodes := values["recovery_codes"]
	assert.False(t, hasCodes, "session must not stash recovery_codes on happy path")
	_, hasRedirect := values["redirect_after_login"]
	assert.False(t, hasRedirect, "session must not stash redirect_after_login on happy path")
}

func TestRecoveryLoginLastCodePreservesSessionRedirect(t *testing.T) {
	h, repo := setupTestApp(t)
	user, _ := repo.CreateUser(context.Background(), "alice")

	svc := &RecoveryService{BcryptCost: bcrypt.MinCost}
	codes, hashes, err := svc.GenerateCodes(CodeCount)
	require.NoError(t, err)
	require.NoError(t, repo.CreateRecoveryCodes(context.Background(), user.ID, hashes[:1]))

	body := strings.NewReader(`{"username":"alice","code":"` + codes[0] + `"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery", body)
	req.Header.Set("Content-Type", "application/json")
	req = session.Inject(req, map[string]any{"redirect_after_login": "/custom-page"})
	rec := httptest.NewRecorder()

	err = h.RecoveryLogin(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"redirect":"/auth/recovery-codes"`)

	values := session.GetValues(req)
	require.NotNil(t, values)
	assert.Equal(t, "/custom-page", values["redirect_after_login"], "original redirect target must survive the regenerate")
}

// --- RegenerateRecoveryCodes tests ---

func TestRegenerateRecoveryCodes(t *testing.T) {
	h, repo := setupTestApp(t)
	user, _ := repo.CreateUser(context.Background(), "alice")

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
	h, _ := setupTestApp(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/recovery-codes", nil)
	req = session.Inject(req, map[string]any{
		"user_id":        "test-user-1",
		"recovery_codes": []string{"code1", "code2"},
	})
	ctx := WithUser(req.Context(), testUserWithID())
	ctx = burrow.WithTemplateExecutor(ctx, rendererTestExecutor())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := h.RecoveryCodesPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "recovery-codes-title")
	assert.Contains(t, rec.Body.String(), "code1")
}

func TestRecoveryCodesPageWithoutCodes(t *testing.T) {
	h, _ := setupTestApp(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/recovery-codes", nil)
	req = requestWithSession(req, testUserWithID())
	rec := httptest.NewRecorder()

	err := h.RecoveryCodesPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/dashboard", rec.Header().Get("Location"))
}

// --- AcknowledgeRecoveryCodes tests ---

func TestAcknowledgeRecoveryCodes(t *testing.T) {
	h, _ := setupTestApp(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery-codes/ack", nil)
	req = session.Inject(req, map[string]any{
		"user_id":        "test-user-1",
		"recovery_codes": []string{"code1"},
	})
	ctx := WithUser(req.Context(), testUserWithID())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := h.AcknowledgeRecoveryCodes(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/dashboard", rec.Header().Get("Location"))
}

func TestAcknowledgeRecoveryCodesWithRedirect(t *testing.T) {
	h, _ := setupTestApp(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery-codes/ack", nil)
	req = session.Inject(req, map[string]any{
		"user_id":              "test-user-1",
		"recovery_codes":       []string{"code1"},
		"redirect_after_login": "/admin/",
	})
	ctx := WithUser(req.Context(), testUserWithID())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := h.AcknowledgeRecoveryCodes(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/admin/", rec.Header().Get("Location"))
}

// --- Email verification tests ---

func TestVerifyPendingPage(t *testing.T) {
	h, _ := setupTestAppEmailMode(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-pending", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.VerifyPendingPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "verify-pending-title")
}

func TestVerifyEmailMissingToken(t *testing.T) {
	h, _ := setupTestAppEmailMode(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-email", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.VerifyEmail(rec, req)

	require.NoError(t, err)
	assert.Contains(t, rec.Body.String(), "verify-error-title")
	assert.Contains(t, rec.Body.String(), "verify-error-missing-token")
}

func TestVerifyEmailInvalidToken(t *testing.T) {
	h, _ := setupTestAppEmailMode(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-email?token=invalid", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.VerifyEmail(rec, req)

	require.NoError(t, err)
	assert.Contains(t, rec.Body.String(), "verify-error-title")
	assert.Contains(t, rec.Body.String(), "verify-error-invalid-token")
}

func TestVerifyEmailExpiredToken(t *testing.T) {
	h, repo := setupTestAppEmailMode(t)
	user, _ := repo.CreateUserWithEmail(context.Background(), "test@example.com")
	tokenHash := HashToken("expiredtoken")
	require.NoError(t, repo.CreateEmailVerificationToken(context.Background(), user.ID, tokenHash, time.Now().Add(-time.Hour)))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-email?token=expiredtoken", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.VerifyEmail(rec, req)

	require.NoError(t, err)
	assert.Contains(t, rec.Body.String(), "verify-error-title")
	assert.Contains(t, rec.Body.String(), "verify-error-token-expired")
}

func TestVerifyEmailSuccess(t *testing.T) {
	h, repo := setupTestAppEmailMode(t)
	user, _ := repo.CreateUserWithEmail(context.Background(), "test@example.com")
	tokenHash := HashToken("validtoken")
	require.NoError(t, repo.CreateEmailVerificationToken(context.Background(), user.ID, tokenHash, time.Now().Add(24*time.Hour)))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-email?token=validtoken", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.VerifyEmail(rec, req)

	require.NoError(t, err)
	assert.Contains(t, rec.Body.String(), "verify-success-title")

	// User should be marked as verified.
	got, _ := repo.GetUserByID(context.Background(), user.ID)
	assert.True(t, got.EmailVerified)
}

// --- ResendVerification tests ---

func TestResendVerificationMissingEmail(t *testing.T) {
	h, _ := setupTestAppEmailMode(t)
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
	h, _ := setupTestAppEmailMode(t)
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
	h, repo := setupTestAppEmailMode(t)

	user, err := repo.CreateUserWithEmail(context.Background(), "verified@example.com")
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
	h, repo := setupTestAppEmailMode(t)

	_, err := repo.CreateUserWithEmail(context.Background(), "test@example.com")
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
	h, _ := setupTestAppEmailMode(t)

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
	h, repo := setupTestApp(t)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "inactive")
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
	h, _ := setupTestApp(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/recovery-codes", nil)
	req = session.Inject(req, map[string]any{
		"user_id":        "test-user-1",
		"recovery_codes": "not-a-slice",
	})
	ctx := WithUser(req.Context(), testUserWithID())
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
	h, _ := setupTestApp(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/recovery-codes/ack", nil)
	req = session.Inject(req, map[string]any{
		"user_id":              "test-user-1",
		"recovery_codes":       []string{"code1"},
		"redirect_after_login": "/custom-redirect",
	})
	ctx := WithUser(req.Context(), testUserWithID())
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
	repo := NewRepository[EmptyProfile](db)
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	emailSvc := &mockEmailService{}
	h := testApp(t, testI18nBundle(t))
	h.repo = repo
	h.webauthn = waSvc
	h.emailService = emailSvc
	h.config = &Config{
		LoginRedirect:       "/dashboard",
		LogoutRedirect:      "/auth/login",
		UseEmail:            true,
		RequireVerification: true,
		InviteOnly:          true,
		BaseURL:             "http://localhost:8080",
	}

	// Create existing user so first-user bypass doesn't apply.
	_, err = repo.CreateUser(context.Background(), "existing")
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
	h, repo := setupTestAppInviteOnly(t)

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
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "register-username-label")
}

// --- UseEmailMode / IsInviteOnly with nil config ---

func TestUseEmailModeNilConfig(t *testing.T) {
	a := &App[EmptyProfile]{config: nil}
	assert.False(t, a.UseEmailMode())
}

func TestIsInviteOnlyNilConfig(t *testing.T) {
	a := &App[EmptyProfile]{config: nil}
	assert.False(t, a.IsInviteOnly())
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
	h, _ := setupTestApp(t)
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
	h, _ := setupTestApp(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/recovery-codes", nil)
	req = session.Inject(req, map[string]any{
		"user_id":        "test-user-1",
		"recovery_codes": []string{},
	})
	ctx := WithUser(req.Context(), testUserWithID())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := h.RecoveryCodesPage(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/dashboard", rec.Header().Get("Location"))
}

// --- VerifyEmail: DB error on MarkEmailVerified ---

func TestVerifyEmailDBErrorOnMarkVerified(t *testing.T) {
	h, repo, closableDB := setupTestAppEmailModeClosable(t)
	ctx := context.Background()

	user, err := repo.CreateUserWithEmail(ctx, "test@example.com")
	require.NoError(t, err)

	tokenHash := HashToken("goodtoken")
	require.NoError(t, repo.CreateEmailVerificationToken(ctx, user.ID, tokenHash, time.Now().Add(24*time.Hour)))

	// Close the DB so MarkEmailVerified fails.
	_ = closableDB.Close()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-email?token=goodtoken", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err = h.VerifyEmail(rec, req)
	require.NoError(t, err)
	// Renders the error page (an HTML page, served with 200).
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "verify-error-title")
}

// --- VerifyEmail: DB error on GetUserByID after verification ---

func TestVerifyEmailDBErrorOnGetUserAfterVerify(t *testing.T) {
	h, repo, _ := setupTestAppEmailModeClosable(t)
	ctx := context.Background()

	user, err := repo.CreateUserWithEmail(ctx, "test@example.com")
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
	// GetUserByID returns ErrNotFound → renders the error page (200).
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "verify-error-title")
}

// --- ResendVerification: user with nil email ---

func TestResendVerificationUserWithNilEmail(t *testing.T) {
	h, repo := setupTestAppEmailMode(t)

	// Create a user via username mode (no email), then look up by email.
	// Since GetUserByEmail would fail for a username-mode user, we need a user
	// whose Email field is nil but is somehow found. This path is defensive;
	// we can test it by creating a user with email, then nullifying it.
	user, err := repo.CreateUserWithEmail(context.Background(), "nullemail@example.com")
	require.NoError(t, err)

	// Nullify the email via Den update.
	reloaded, err := repo.GetUserByID(context.Background(), user.ID)
	require.NoError(t, err)
	reloaded.Email = nil
	require.NoError(t, den.Save(context.Background(), repo.db, reloaded))

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
	h, repo, closableDB := setupTestAppClosable(t)
	user, _ := repo.CreateUser(context.Background(), "bob")
	cred1 := &Credential{UserID: user.ID, CredentialID: []byte("c1"), PublicKey: []byte("k1"), Name: "P1"}
	cred2 := &Credential{UserID: user.ID, CredentialID: []byte("c2"), PublicKey: []byte("k2"), Name: "P2"}
	require.NoError(t, repo.CreateCredential(context.Background(), cred1))
	require.NoError(t, repo.CreateCredential(context.Background(), cred2))

	// Close DB so CountUserCredentials fails.
	_ = closableDB.Close()

	router := chi.NewRouter()
	router.Delete("/auth/credentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		r = requestWithSession(r, user)
		_ = h.DeleteCredential(w, r)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/auth/credentials/"+cred1.ID, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "database error")
}

// --- CredentialsPage: DB error on GetCredentialsByUserID ---

func TestCredentialsPageDBError(t *testing.T) {
	h, repo, closableDB := setupTestAppClosable(t)
	user, _ := repo.CreateUser(context.Background(), "carol")

	// Close DB so GetCredentialsByUserID fails.
	_ = closableDB.Close()

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
	h, repo, closableDB := setupTestAppClosable(t)
	user, _ := repo.CreateUser(context.Background(), "eve")

	// Close DB so generateAndStoreRecoveryCodes fails (DeleteRecoveryCodes fails).
	_ = closableDB.Close()

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
	h, _, closableDB := setupTestAppClosable(t)

	// Close DB so CountUsers fails.
	_ = closableDB.Close()

	isFirst, err := h.isFirstUser(context.Background())
	require.Error(t, err)
	assert.False(t, isFirst)
}

// --- RecoveryLogin: DB error on ValidateAndUseRecoveryCode ---

func TestRecoveryLoginDBErrorOnValidation(t *testing.T) {
	h, repo, closableDB := setupTestAppClosable(t)
	user, _ := repo.CreateUser(context.Background(), "frank")

	svc := &RecoveryService{BcryptCost: bcrypt.MinCost}
	codes, hashes, err := svc.GenerateCodes(CodeCount)
	require.NoError(t, err)
	require.NoError(t, repo.CreateRecoveryCodes(context.Background(), user.ID, hashes))

	// Close DB so ValidateAndUseRecoveryCode fails.
	_ = closableDB.Close()

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
	h, repo, closableDB := setupTestAppEmailModeClosable(t)

	_, err := repo.CreateUserWithEmail(context.Background(), "test@example.com")
	require.NoError(t, err)

	// Close DB so CreateEmailVerificationToken fails.
	_ = closableDB.Close()

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
	h, repo := setupTestApp(t)
	user, _ := repo.CreateUser(context.Background(), "charlie")

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
	h, repo := setupTestApp(t)
	user, _ := repo.CreateUser(context.Background(), "alice")

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
	h, repo := setupTestApp(t)
	user, err := repo.CreateUser(context.Background(), "dave")
	require.NoError(t, err)

	cred := &Credential{UserID: user.ID, CredentialID: []byte("c1"), PublicKey: []byte("k1"), Name: "My Key"}
	require.NoError(t, repo.CreateCredential(context.Background(), cred))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/credentials", nil)
	req = requestWithSession(req, user)
	rec := httptest.NewRecorder()

	err = h.CredentialsPage(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "My Key")
}

// --- AcknowledgeRecoveryCodes: no session middleware ---

func TestAcknowledgeRecoveryCodesNoSession(t *testing.T) {
	h, _ := setupTestApp(t)

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
	h, repo := setupTestApp(t)
	user, _ := repo.CreateUser(context.Background(), "nosession")

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

// --- ResendVerification: full path with existing old tokens ---

func TestResendVerificationDeletesOldTokens(t *testing.T) {
	h, repo := setupTestAppEmailMode(t)
	ctx := context.Background()

	user, err := repo.CreateUserWithEmail(ctx, "test@example.com")
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
