package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"codeberg.org/oliverandrich/go-webapp-template/contrib/session"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type mockRenderer struct {
	lastMethod string
}

func (m *mockRenderer) RegisterPage(c *echo.Context, _, _ bool, _, _ string) error {
	m.lastMethod = "RegisterPage"
	return c.String(http.StatusOK, "register")
}

func (m *mockRenderer) LoginPage(c *echo.Context, _ string) error {
	m.lastMethod = "LoginPage"
	return c.String(http.StatusOK, "login")
}

func (m *mockRenderer) CredentialsPage(c *echo.Context, _ []Credential) error {
	m.lastMethod = "CredentialsPage"
	return c.String(http.StatusOK, "credentials")
}

func (m *mockRenderer) RecoveryPage(c *echo.Context, _ string) error {
	m.lastMethod = "RecoveryPage"
	return c.String(http.StatusOK, "recovery")
}

func (m *mockRenderer) VerifyPendingPage(c *echo.Context) error {
	m.lastMethod = "VerifyPendingPage"
	return c.String(http.StatusOK, "verify-pending")
}

func (m *mockRenderer) VerifyEmailSuccess(c *echo.Context) error {
	m.lastMethod = "VerifyEmailSuccess"
	return c.String(http.StatusOK, "verify-success")
}

func (m *mockRenderer) VerifyEmailError(c *echo.Context, _ string) error {
	m.lastMethod = "VerifyEmailError"
	return c.String(http.StatusBadRequest, "verify-error")
}

func (m *mockRenderer) InvitesPage(c *echo.Context, _ []Invite, _ string, _ bool) error {
	m.lastMethod = "InvitesPage"
	return c.String(http.StatusOK, "invites")
}

type mockEmailService struct {
	sendCalled bool
}

func (m *mockEmailService) GenerateToken() (string, string, time.Time, error) {
	return "plaintoken", HashToken("plaintoken"), time.Now().Add(24 * time.Hour), nil
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

func newTestHandlers(t *testing.T) (*Handlers, *Repository, *mockRenderer) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository(db)
	renderer := &mockRenderer{}
	waSvc, err := NewWebAuthnService("Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	h := NewHandlers(repo, waSvc, nil, renderer, &Config{
		LoginRedirect: "/dashboard",
	})
	return h, repo, renderer
}

func newTestHandlersEmailMode(t *testing.T) (*Handlers, *Repository, *mockRenderer) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository(db)
	renderer := &mockRenderer{}
	waSvc, err := NewWebAuthnService("Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	h := NewHandlers(repo, waSvc, &mockEmailService{}, renderer, &Config{
		LoginRedirect:       "/dashboard",
		UseEmail:            true,
		RequireVerification: true,
	})
	return h, repo, renderer
}

func newTestHandlersInviteOnly(t *testing.T) (*Handlers, *Repository, *mockRenderer) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository(db)
	renderer := &mockRenderer{}
	waSvc, err := NewWebAuthnService("Test App", "localhost", "http://localhost:8080")
	require.NoError(t, err)

	h := NewHandlers(repo, waSvc, nil, renderer, &Config{
		LoginRedirect: "/dashboard",
		InviteOnly:    true,
	})
	return h, repo, renderer
}

func echoContext(e *echo.Echo, req *http.Request, rec *httptest.ResponseRecorder, user *User) *echo.Context {
	c := e.NewContext(req, rec)
	session.Inject(c, nil)
	if user != nil {
		SetUser(c, user)
	}
	return c
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
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/register", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.RegisterPage(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "RegisterPage", r.lastMethod)
}

func TestRegisterPageInviteOnlyNoToken(t *testing.T) {
	h, _, r := newTestHandlersInviteOnly(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/register", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.RegisterPage(c)

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

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/register?invite=validtoken", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.RegisterPage(c)

	require.NoError(t, err)
	assert.Equal(t, "RegisterPage", r.lastMethod)
}

// --- RegisterBegin tests ---

func TestRegisterBeginUsernameMode(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	e := echo.New()
	body := strings.NewReader(`{"username":"newuser"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.RegisterBegin(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "publicKey")
	assert.Contains(t, rec.Body.String(), "user_id")
}

func TestRegisterBeginMissingUsername(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	e := echo.New()
	body := strings.NewReader(`{"username":""}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.RegisterBegin(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "username is required")
}

func TestRegisterBeginUsernameExists(t *testing.T) {
	h, repo, _ := newTestHandlers(t)

	_, err := repo.CreateUser(context.Background(), "taken", "")
	require.NoError(t, err)

	e := echo.New()
	body := strings.NewReader(`{"username":"taken"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err = h.RegisterBegin(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "registration failed")
}

func TestRegisterBeginInvalidJSON(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	e := echo.New()
	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.RegisterBegin(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRegisterBeginEmailMode(t *testing.T) {
	h, _, _ := newTestHandlersEmailMode(t)
	e := echo.New()
	body := strings.NewReader(`{"email":"test@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.RegisterBegin(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "publicKey")
}

func TestRegisterBeginEmailModeMissingEmail(t *testing.T) {
	h, _, _ := newTestHandlersEmailMode(t)
	e := echo.New()
	body := strings.NewReader(`{"email":""}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.RegisterBegin(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "email is required")
}

func TestRegisterBeginEmailModeEmailExists(t *testing.T) {
	h, repo, _ := newTestHandlersEmailMode(t)

	_, err := repo.CreateUserWithEmail(context.Background(), "taken@example.com", "")
	require.NoError(t, err)

	e := echo.New()
	body := strings.NewReader(`{"email":"taken@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err = h.RegisterBegin(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestRegisterBeginInviteOnlyNoToken(t *testing.T) {
	h, repo, _ := newTestHandlersInviteOnly(t)

	// Create a user so first-user bypass doesn't apply.
	_, err := repo.CreateUser(context.Background(), "existing", "")
	require.NoError(t, err)

	e := echo.New()
	body := strings.NewReader(`{"username":"newuser"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err = h.RegisterBegin(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRegisterBeginInviteOnlyFirstUserBypass(t *testing.T) {
	h, _, _ := newTestHandlersInviteOnly(t)

	// No users exist - first user bypasses invite requirement.
	e := echo.New()
	body := strings.NewReader(`{"username":"firstuser"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.RegisterBegin(c)

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

	e := echo.New()
	body := strings.NewReader(`{"username":"newuser","invite":"invitetoken"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err = h.RegisterBegin(c)

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

	e := echo.New()
	body := strings.NewReader(`{"username":"newuser","invite":"expiredtoken"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err = h.RegisterBegin(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// --- RegisterFinish tests ---

func TestRegisterFinishInvalidUserID(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/auth/register/finish?user_id=invalid", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.RegisterFinish(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid user_id")
}

func TestRegisterFinishSessionExpired(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/auth/register/finish?user_id=99999", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.RegisterFinish(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- LoginPage tests ---

func TestLoginPage(t *testing.T) {
	h, _, r := newTestHandlers(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.LoginPage(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "LoginPage", r.lastMethod)
}

// --- LoginBegin tests ---

func TestLoginBegin(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/auth/login/begin", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.LoginBegin(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "publicKey")
	assert.Contains(t, rec.Body.String(), "session_id")
}

// --- LoginFinish tests ---

func TestLoginFinishMissingSessionID(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/auth/login/finish", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.LoginFinish(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "session_id is required")
}

func TestLoginFinishSessionExpired(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/auth/login/finish?session_id=nonexistent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.LoginFinish(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "login session expired")
}

// --- Logout tests ---

func TestLogout(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.Logout(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/", rec.Header().Get("Location"))

	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, -1, cookies[0].MaxAge)
}

// --- CredentialsPage tests ---

func TestCredentialsPage(t *testing.T) {
	h, repo, r := newTestHandlers(t)
	user, err := repo.CreateUser(context.Background(), "alice", "")
	require.NoError(t, err)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/credentials", nil)
	rec := httptest.NewRecorder()
	c := echoContext(e, req, rec, user)

	err = h.CredentialsPage(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "CredentialsPage", r.lastMethod)
}

// --- DeleteCredential tests ---

func TestDeleteCredentialInvalidID(t *testing.T) {
	h, repo, _ := newTestHandlers(t)
	user, _ := repo.CreateUser(context.Background(), "alice", "")

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/auth/credentials/invalid", nil)
	rec := httptest.NewRecorder()
	c := echoContext(e, req, rec, user)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "invalid"}})

	err := h.DeleteCredential(c)

	require.NoError(t, err)
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

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/auth/credentials/"+strconv.FormatInt(cred.ID, 10), nil)
	rec := httptest.NewRecorder()
	c := echoContext(e, req, rec, user)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: strconv.FormatInt(cred.ID, 10)}})

	err := h.DeleteCredential(c)

	require.NoError(t, err)
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

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/auth/credentials/"+strconv.FormatInt(cred1.ID, 10), nil)
	rec := httptest.NewRecorder()
	c := echoContext(e, req, rec, user)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: strconv.FormatInt(cred1.ID, 10)}})

	err := h.DeleteCredential(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- RecoveryPage tests ---

func TestRecoveryPage(t *testing.T) {
	h, _, r := newTestHandlers(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/recovery", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.RecoveryPage(c)

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
			e := echo.New()
			body := strings.NewReader(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/auth/recovery", body)
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			session.Inject(c, nil)

			err := h.RecoveryLogin(c)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestRecoveryLoginUserNotFound(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	e := echo.New()
	body := strings.NewReader(`{"username":"nonexistent","code":"abcd-efgh-ijkl"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.RecoveryLogin(c)

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

	e := echo.New()
	body := strings.NewReader(`{"username":"alice","code":"wrong-code-here"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err = h.RecoveryLogin(c)

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

	e := echo.New()
	body := strings.NewReader(`{"username":"alice","code":"` + codes[0] + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err = h.RecoveryLogin(c)

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

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery-codes/regenerate", nil)
	rec := httptest.NewRecorder()
	c := echoContext(e, req, rec, user)

	err = h.RegenerateRecoveryCodes(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
	assert.Contains(t, rec.Body.String(), `"recovery_codes"`)
}

// --- Email verification tests ---

func TestVerifyPendingPage(t *testing.T) {
	h, _, r := newTestHandlersEmailMode(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/verify-pending", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.VerifyPendingPage(c)

	require.NoError(t, err)
	assert.Equal(t, "VerifyPendingPage", r.lastMethod)
}

func TestVerifyEmailMissingToken(t *testing.T) {
	h, _, r := newTestHandlersEmailMode(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/verify-email", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.VerifyEmail(c)

	require.NoError(t, err)
	assert.Equal(t, "VerifyEmailError", r.lastMethod)
}

func TestVerifyEmailInvalidToken(t *testing.T) {
	h, _, r := newTestHandlersEmailMode(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/verify-email?token=invalid", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.VerifyEmail(c)

	require.NoError(t, err)
	assert.Equal(t, "VerifyEmailError", r.lastMethod)
}

func TestVerifyEmailExpiredToken(t *testing.T) {
	h, repo, r := newTestHandlersEmailMode(t)
	user, _ := repo.CreateUserWithEmail(context.Background(), "test@example.com", "")
	tokenHash := HashToken("expiredtoken")
	require.NoError(t, repo.CreateEmailVerificationToken(context.Background(), user.ID, tokenHash, time.Now().Add(-time.Hour)))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/verify-email?token=expiredtoken", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.VerifyEmail(c)

	require.NoError(t, err)
	assert.Equal(t, "VerifyEmailError", r.lastMethod)
}

func TestVerifyEmailSuccess(t *testing.T) {
	h, repo, r := newTestHandlersEmailMode(t)
	user, _ := repo.CreateUserWithEmail(context.Background(), "test@example.com", "")
	tokenHash := HashToken("validtoken")
	require.NoError(t, repo.CreateEmailVerificationToken(context.Background(), user.ID, tokenHash, time.Now().Add(24*time.Hour)))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/verify-email?token=validtoken", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.VerifyEmail(c)

	require.NoError(t, err)
	assert.Equal(t, "VerifyEmailSuccess", r.lastMethod)

	// User should be marked as verified.
	got, _ := repo.GetUserByID(context.Background(), user.ID)
	assert.True(t, got.EmailVerified)
}

// --- ResendVerification tests ---

func TestResendVerificationMissingEmail(t *testing.T) {
	h, _, _ := newTestHandlersEmailMode(t)
	e := echo.New()
	body := strings.NewReader(`{"email":""}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/resend-verification", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.ResendVerification(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestResendVerificationNonexistentEmail(t *testing.T) {
	h, _, _ := newTestHandlersEmailMode(t)
	e := echo.New()
	body := strings.NewReader(`{"email":"nobody@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/resend-verification", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.ResendVerification(c)

	require.NoError(t, err)
	// Should still return OK (don't reveal if email exists).
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- InvitesPage tests ---

func TestInvitesPage(t *testing.T) {
	h, _, r := newTestHandlers(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/invites", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)

	err := h.InvitesPage(c)

	require.NoError(t, err)
	assert.Equal(t, "InvitesPage", r.lastMethod)
}

// --- CreateInvite tests ---

func TestCreateInvite(t *testing.T) {
	h, repo, r := newTestHandlers(t)
	user, _ := repo.CreateUser(context.Background(), "admin", "")

	e := echo.New()
	body := strings.NewReader(`email=invitee@example.com`)
	req := httptest.NewRequest(http.MethodPost, "/admin/invites", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	c := echoContext(e, req, rec, user)

	err := h.CreateInvite(c)

	require.NoError(t, err)
	assert.Equal(t, "InvitesPage", r.lastMethod)

	invites, err := repo.ListInvites(context.Background())
	require.NoError(t, err)
	assert.Len(t, invites, 1)
	assert.Equal(t, "invitee@example.com", invites[0].Email)
}

func TestCreateInviteNoAuth(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	e := echo.New()
	body := strings.NewReader(`email=test@example.com`)
	req := httptest.NewRequest(http.MethodPost, "/admin/invites", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil) // No user in context.

	err := h.CreateInvite(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- DeleteInvite tests ---

func TestDeleteInviteInvalidID(t *testing.T) {
	h, _, _ := newTestHandlers(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/admin/invites/invalid", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "invalid"}})

	err := h.DeleteInvite(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeleteInviteSuccess(t *testing.T) {
	h, repo, _ := newTestHandlers(t)
	invite := &Invite{
		Email:     "delete@example.com",
		TokenHash: "deletehash",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, repo.CreateInvite(context.Background(), invite))

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/admin/invites/"+strconv.FormatInt(invite.ID, 10), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	session.Inject(c, nil)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: strconv.FormatInt(invite.ID, 10)}})

	err := h.DeleteInvite(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}
