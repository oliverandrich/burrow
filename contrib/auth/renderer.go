package auth

import (
	"embed"
	"net/http"
	"strconv"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/i18n"
	"github.com/a-h/templ"
)

//go:embed static
var staticFS embed.FS

// DefaultRenderer returns a Renderer that uses the built-in Templ templates.
// Templates read burrow.Layout(ctx) at render time: if a layout is set,
// page content is wrapped in it; otherwise bare content is rendered.
func DefaultRenderer() Renderer {
	return &defaultRenderer{}
}

// DefaultAdminRenderer returns an AdminRenderer that uses the built-in Templ
// templates for admin pages (users, user detail, invites).
func DefaultAdminRenderer() AdminRenderer {
	return &defaultAdminRenderer{}
}

// defaultRenderer implements Renderer using built-in Templ templates.
type defaultRenderer struct{}

func (d *defaultRenderer) LoginPage(w http.ResponseWriter, r *http.Request, loginRedirect string) error {
	return renderWithLayout(w, r, i18n.T(r.Context(), "login-title"), loginPage(loginRedirect))
}

func (d *defaultRenderer) RegisterPage(w http.ResponseWriter, r *http.Request, useEmail, inviteOnly bool, email, invite string) error {
	return renderWithLayout(w, r, i18n.T(r.Context(), "register-title"), registerPage(useEmail, inviteOnly, email, invite))
}

func (d *defaultRenderer) CredentialsPage(w http.ResponseWriter, r *http.Request, creds []Credential) error {
	return renderWithLayout(w, r, i18n.T(r.Context(), "credentials-title"), credentialsPage(creds))
}

func (d *defaultRenderer) RecoveryPage(w http.ResponseWriter, r *http.Request, loginRedirect string) error {
	return renderWithLayout(w, r, i18n.T(r.Context(), "recovery-title"), recoveryPage(loginRedirect))
}

func (d *defaultRenderer) RecoveryCodesPage(w http.ResponseWriter, r *http.Request, codes []string) error {
	return renderWithLayout(w, r, i18n.T(r.Context(), "recovery-codes-title"), recoveryCodesPage(codes))
}

func (d *defaultRenderer) VerifyPendingPage(w http.ResponseWriter, r *http.Request) error {
	return renderWithLayout(w, r, i18n.T(r.Context(), "verify-pending-title"), verifyPendingPage())
}

func (d *defaultRenderer) VerifyEmailSuccess(w http.ResponseWriter, r *http.Request) error {
	return renderWithLayout(w, r, i18n.T(r.Context(), "verify-success-title"), verifyEmailSuccessPage())
}

func (d *defaultRenderer) VerifyEmailError(w http.ResponseWriter, r *http.Request, errorCode string) error {
	return renderWithLayout(w, r, i18n.T(r.Context(), "verify-error-title"), verifyEmailErrorPage(errorCode))
}

// defaultAdminRenderer implements AdminRenderer using built-in Templ templates.
type defaultAdminRenderer struct{}

func (d *defaultAdminRenderer) AdminUsersPage(w http.ResponseWriter, r *http.Request, users []User) error {
	return renderWithLayout(w, r, i18n.T(r.Context(), "admin-users-title"), adminUsersPage(users))
}

func (d *defaultAdminRenderer) AdminUserDetailPage(w http.ResponseWriter, r *http.Request, user *User) error {
	return renderWithLayout(w, r, i18n.T(r.Context(), "admin-user-detail-title")+user.Username, adminUserDetailPage(user))
}

func (d *defaultAdminRenderer) AdminInvitesPage(w http.ResponseWriter, r *http.Request, invites []Invite, createdURL string, useEmail bool) error {
	return renderWithLayout(w, r, i18n.T(r.Context(), "admin-invites-title"), adminInvitesPage(invites, createdURL, useEmail))
}

// renderWithLayout wraps content in the layout from context, or renders bare content.
func renderWithLayout(w http.ResponseWriter, r *http.Request, title string, content templ.Component) error {
	layout := burrow.Layout(r.Context())
	if layout != nil {
		return burrow.Render(w, r, http.StatusOK, layout(title, content))
	}
	return burrow.Render(w, r, http.StatusOK, content)
}

// Template helper functions used by the Templ templates.

// itoa converts an int64 to a string for use in template attributes.
func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}

// credName returns a display name for a credential.
func credName(cred Credential) string {
	if cred.Name != "" {
		return cred.Name
	}
	return "Passkey"
}
