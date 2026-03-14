package auth

import (
	"net/http"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/i18n"
)

// DefaultAuthLayout returns the template name for the built-in auth layout.
func DefaultAuthLayout() string {
	return "auth/layout"
}

// DefaultRenderer returns the default Renderer that uses the built-in HTML
// templates. Templates use burrow.RenderTemplate which reads layout from
// context: if a layout is set, page content is wrapped in it; otherwise
// bare content is rendered.
func DefaultRenderer() Renderer {
	return &defaultRenderer{}
}

// defaultRenderer implements Renderer using built-in HTML templates.
type defaultRenderer struct{}

func (d *defaultRenderer) LoginPage(w http.ResponseWriter, r *http.Request, loginRedirect string) error {
	return renderCentered(w, r, i18n.T(r.Context(), "login-title"), "auth/login", map[string]any{
		"LoginRedirect": loginRedirect,
	})
}

func (d *defaultRenderer) RegisterPage(w http.ResponseWriter, r *http.Request, useEmail, inviteOnly bool, email, invite string) error {
	return renderCard(w, r, i18n.T(r.Context(), "register-title"), "", "auth/register", map[string]any{
		"UseEmail":   useEmail,
		"InviteOnly": inviteOnly,
		"Email":      email,
		"Invite":     invite,
	})
}

func (d *defaultRenderer) CredentialsPage(w http.ResponseWriter, r *http.Request, creds []Credential) error {
	return renderCard(w, r, i18n.T(r.Context(), "credentials-title"), i18n.T(r.Context(), "credentials-title"), "auth/credentials", map[string]any{
		"Creds": creds,
	})
}

func (d *defaultRenderer) RecoveryPage(w http.ResponseWriter, r *http.Request, loginRedirect string) error {
	return renderCard(w, r, i18n.T(r.Context(), "recovery-title"), "", "auth/recovery", map[string]any{
		"LoginRedirect": loginRedirect,
	})
}

func (d *defaultRenderer) RecoveryCodesPage(w http.ResponseWriter, r *http.Request, codes []string) error {
	return renderCard(w, r, i18n.T(r.Context(), "recovery-codes-title"), i18n.T(r.Context(), "recovery-codes-title"), "auth/recovery_codes", map[string]any{
		"Codes": codes,
	})
}

func (d *defaultRenderer) VerifyPendingPage(w http.ResponseWriter, r *http.Request) error {
	return renderCard(w, r, i18n.T(r.Context(), "verify-pending-title"), i18n.T(r.Context(), "verify-pending-title"), "auth/verify_pending", nil)
}

func (d *defaultRenderer) VerifyEmailSuccess(w http.ResponseWriter, r *http.Request) error {
	return renderCard(w, r, i18n.T(r.Context(), "verify-success-title"), i18n.T(r.Context(), "verify-success-title"), "auth/verify_success", nil)
}

func (d *defaultRenderer) VerifyEmailError(w http.ResponseWriter, r *http.Request, errorCode string) error {
	return renderCard(w, r, i18n.T(r.Context(), "verify-error-title"), i18n.T(r.Context(), "verify-error-title"), "auth/verify_error", map[string]any{
		"ErrorCode": errorCode,
	})
}

// renderCentered renders a template wrapped in the auth/centered layout (no card).
func renderCentered(w http.ResponseWriter, r *http.Request, title, name string, data map[string]any) error {
	exec := burrow.TemplateExec(r.Context())
	if exec == nil {
		return burrow.Render(w, r, http.StatusOK, name, data)
	}

	inner, err := exec(r, name, data)
	if err != nil {
		return err
	}

	centeredData := map[string]any{"Content": inner, "Title": title}
	return burrow.Render(w, r, http.StatusOK, "auth/centered", centeredData)
}

// renderCard renders a template wrapped in the auth/card layout.
func renderCard(w http.ResponseWriter, r *http.Request, title, cardTitle, name string, data map[string]any) error {
	exec := burrow.TemplateExec(r.Context())
	if exec == nil {
		return burrow.Render(w, r, http.StatusOK, name, data)
	}

	inner, err := exec(r, name, data)
	if err != nil {
		return err
	}

	cardData := map[string]any{"Content": inner, "Title": title, "CardTitle": cardTitle}
	return burrow.Render(w, r, http.StatusOK, "auth/card", cardData)
}
