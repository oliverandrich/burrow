package templates

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"codeberg.org/oliverandrich/burrow/contrib/csrf"
	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ auth.Renderer      = (*defaultRenderer)(nil)
	_ auth.AdminRenderer = (*defaultAdminRenderer)(nil)
)

func TestDefaultRendererLoginPage(t *testing.T) {
	r := DefaultRenderer()
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()

	err := r.LoginPage(rec, req, "/dashboard")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "login-title")
	assert.Contains(t, body, "login-button")
	assert.Contains(t, body, "card shadow-sm", "login page should be wrapped in a card")
}

func TestDefaultRendererRegisterPage(t *testing.T) {
	r := DefaultRenderer()
	req := httptest.NewRequest(http.MethodGet, "/auth/register", nil)
	rec := httptest.NewRecorder()

	err := r.RegisterPage(rec, req, false, false, "", "")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "register-title")
	assert.Contains(t, body, "register-username-label")
	assert.Contains(t, body, "card shadow-sm", "register page should be wrapped in a card")
}

func TestDefaultRendererRegisterPageEmailMode(t *testing.T) {
	r := DefaultRenderer()
	req := httptest.NewRequest(http.MethodGet, "/auth/register", nil)
	rec := httptest.NewRecorder()

	err := r.RegisterPage(rec, req, true, false, "", "")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "register-email-label")
	assert.NotContains(t, rec.Body.String(), "register-username-label")
}

func TestDefaultRendererCredentialsPage(t *testing.T) {
	r := DefaultRenderer()
	creds := []auth.Credential{
		{ID: 1, Name: "My Passkey"},
	}
	req := httptest.NewRequest(http.MethodGet, "/auth/credentials", nil)
	rec := httptest.NewRecorder()

	err := r.CredentialsPage(rec, req, creds)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "My Passkey")
	assert.Contains(t, body, "card shadow-sm", "credentials page should be wrapped in a card")
}

func TestDefaultRendererRecoveryPage(t *testing.T) {
	r := DefaultRenderer()
	req := httptest.NewRequest(http.MethodGet, "/auth/recovery", nil)
	rec := httptest.NewRecorder()

	err := r.RecoveryPage(rec, req, "/dashboard")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "recovery-title")
	assert.Contains(t, body, "card shadow-sm", "recovery page should be wrapped in a card")
}

func TestDefaultRendererRecoveryCodesPage(t *testing.T) {
	r := DefaultRenderer()
	codes := []string{"aaaa-bbbb-cccc", "dddd-eeee-ffff"}
	req := httptest.NewRequest(http.MethodGet, "/auth/recovery-codes", nil)
	rec := httptest.NewRecorder()

	err := r.RecoveryCodesPage(rec, req, codes)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "recovery-codes-title")
	assert.Contains(t, body, "aaaa-bbbb-cccc")
	assert.Contains(t, body, "dddd-eeee-ffff")
	assert.Contains(t, body, "card shadow-sm", "recovery codes page should be wrapped in a card")
	assert.Contains(t, body, "/auth/recovery-codes/ack")
}

func TestDefaultRendererVerifyPendingPage(t *testing.T) {
	r := DefaultRenderer()
	req := httptest.NewRequest(http.MethodGet, "/auth/verify-pending", nil)
	rec := httptest.NewRecorder()

	err := r.VerifyPendingPage(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "verify-pending-title")
	assert.Contains(t, body, "card shadow-sm", "verify pending page should be wrapped in a card")
}

func TestDefaultRendererVerifyEmailSuccess(t *testing.T) {
	r := DefaultRenderer()
	req := httptest.NewRequest(http.MethodGet, "/auth/verify-email", nil)
	rec := httptest.NewRecorder()

	err := r.VerifyEmailSuccess(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "verify-success-title")
	assert.Contains(t, body, "card shadow-sm", "verify email success page should be wrapped in a card")
}

func TestDefaultRendererVerifyEmailError(t *testing.T) {
	r := DefaultRenderer()
	req := httptest.NewRequest(http.MethodGet, "/auth/verify-email", nil)
	rec := httptest.NewRecorder()

	err := r.VerifyEmailError(rec, req, "invalid_token")

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "verify-error-title")
	assert.Contains(t, body, "verify-error-invalid-token")
	assert.Contains(t, body, "card shadow-sm", "verify email error page should be wrapped in a card")
}

func TestDefaultRendererWithLayout(t *testing.T) {
	r := DefaultRenderer()
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	// Set a layout in context.
	ctx := burrow.WithLayout(req.Context(), func(title string, content templ.Component) templ.Component {
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			_, _ = io.WriteString(w, "<layout-wrapper>")
			if err := content.Render(ctx, w); err != nil {
				return err
			}
			_, _ = io.WriteString(w, "</layout-wrapper>")
			return nil
		})
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := r.LoginPage(rec, req, "/dashboard")

	require.NoError(t, err)
	assert.Contains(t, rec.Body.String(), "<layout-wrapper>")
	assert.Contains(t, rec.Body.String(), "login-button")
	assert.Contains(t, rec.Body.String(), "</layout-wrapper>")
}

func TestDefaultRendererIncludesCSRFToken(t *testing.T) {
	r := DefaultRenderer()
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	ctx := csrf.WithToken(req.Context(), "test-csrf-token-value")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := r.LoginPage(rec, req, "/dashboard")

	require.NoError(t, err)
	body := rec.Body.String()
	assert.Contains(t, body, `id="csrf-token"`)
	assert.Contains(t, body, "test-csrf-token-value")
}

func TestDefaultAdminRendererIncludesCSRFToken(t *testing.T) {
	r := DefaultAdminRenderer()
	user := &auth.User{ID: 1, Username: "alice", Role: auth.RoleAdmin}
	req := httptest.NewRequest(http.MethodGet, "/admin/users/1", nil)
	ctx := csrf.WithToken(req.Context(), "admin-csrf-token")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	err := r.AdminUserDetailPage(rec, req, user)

	require.NoError(t, err)
	body := rec.Body.String()
	assert.Contains(t, body, `name="gorilla.csrf.Token"`)
	assert.Contains(t, body, "admin-csrf-token")
}

func TestDefaultAdminRendererUsersPage(t *testing.T) {
	r := DefaultAdminRenderer()
	users := []auth.User{
		{ID: 1, Username: "alice", Role: auth.RoleUser},
		{ID: 2, Username: "bob", Role: auth.RoleAdmin},
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	rec := httptest.NewRecorder()

	err := r.AdminUsersPage(rec, req, users)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "alice")
	assert.Contains(t, rec.Body.String(), "bob")
}

func TestDefaultAdminRendererUserDetailPage(t *testing.T) {
	r := DefaultAdminRenderer()
	user := &auth.User{ID: 1, Username: "alice", Role: auth.RoleAdmin, Name: "Alice"}
	req := httptest.NewRequest(http.MethodGet, "/admin/users/1", nil)
	rec := httptest.NewRecorder()

	err := r.AdminUserDetailPage(rec, req, user)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "alice")
	assert.Contains(t, rec.Body.String(), "Alice")
}

func TestDefaultAdminRendererInvitesPage(t *testing.T) {
	r := DefaultAdminRenderer()
	invites := []auth.Invite{
		{ID: 1, Label: "John Doe", Email: "test@example.com", ExpiresAt: time.Now().Add(time.Hour)},
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/invites", nil)
	rec := httptest.NewRecorder()

	err := r.AdminInvitesPage(rec, req, invites, "", false)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "John Doe")
	assert.NotContains(t, body, "test@example.com", "email column should be hidden when useEmail is false")
}

func TestDefaultAdminRendererInvitesPageWithCreatedURL(t *testing.T) {
	r := DefaultAdminRenderer()
	req := httptest.NewRequest(http.MethodGet, "/admin/invites", nil)
	rec := httptest.NewRecorder()

	err := r.AdminInvitesPage(rec, req, nil, "http://localhost/auth/register?invite=abc123", false)

	require.NoError(t, err)
	body := rec.Body.String()
	assert.Contains(t, body, "admin-invites-created")
	assert.Contains(t, body, "http://localhost/auth/register?invite=abc123")
	assert.Contains(t, body, "admin-invites-copy")
}
