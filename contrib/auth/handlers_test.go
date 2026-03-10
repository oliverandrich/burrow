package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/i18n"
	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func (m *mockRenderer) VerifyEmailSuccess(w http.ResponseWriter, _ *http.Request) error {
	m.lastMethod = "VerifyEmailSuccess"
	return burrow.Text(w, http.StatusOK, "verify-success")
}

func (m *mockRenderer) VerifyEmailError(w http.ResponseWriter, _ *http.Request, _ string) error {
	m.lastMethod = "VerifyEmailError"
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

func testI18nApp(t *testing.T) *i18n.App {
	t.Helper()
	app, err := i18n.NewTestApp("en", translationFS)
	require.NoError(t, err)
	return app
}

func newTestHandlers(t *testing.T) (*Handlers, *Repository, *mockRenderer) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository(db)
	renderer := &mockRenderer{}
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	h := NewHandlers(repo, waSvc, nil, renderer, &Config{
		LoginRedirect:  "/dashboard",
		LogoutRedirect: "/auth/login",
	}, testI18nApp(t))
	return h, repo, renderer
}

func newTestHandlersEmailMode(t *testing.T) (*Handlers, *Repository, *mockRenderer) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository(db)
	renderer := &mockRenderer{}
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	h := NewHandlers(repo, waSvc, &mockEmailService{}, renderer, &Config{
		LoginRedirect:       "/dashboard",
		LogoutRedirect:      "/auth/login",
		UseEmail:            true,
		RequireVerification: true,
		BaseURL:             "http://localhost:8080",
	}, testI18nApp(t))
	return h, repo, renderer
}

func newTestHandlersInviteOnly(t *testing.T) (*Handlers, *Repository, *mockRenderer) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository(db)
	renderer := &mockRenderer{}
	waSvc, err := NewWebAuthnService(t.Context(), "Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	h := NewHandlers(repo, waSvc, nil, renderer, &Config{
		LoginRedirect:  "/dashboard",
		LogoutRedirect: "/auth/login",
		InviteOnly:     true,
	}, testI18nApp(t))
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
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "registration failed")
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
	assert.Equal(t, http.StatusConflict, rec.Code)
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

	h := NewHandlers(repo, waSvc, nil, renderer, &Config{
		LoginRedirect:  "/dashboard",
		LogoutRedirect: "/goodbye",
	}, testI18nApp(t))

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

	svc := NewRecoveryService()
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

	svc := NewRecoveryService()
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

	svc := NewRecoveryService()
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
	assert.Equal(t, "VerifyEmailError", r.lastMethod)
}

func TestVerifyEmailInvalidToken(t *testing.T) {
	h, _, r := newTestHandlersEmailMode(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/verify-email?token=invalid", nil)
	req = requestWithSession(req, nil)
	rec := httptest.NewRecorder()

	err := h.VerifyEmail(rec, req)

	require.NoError(t, err)
	assert.Equal(t, "VerifyEmailError", r.lastMethod)
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
	assert.Equal(t, "VerifyEmailError", r.lastMethod)
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
	assert.Equal(t, "VerifyEmailSuccess", r.lastMethod)

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
