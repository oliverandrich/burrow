package auth

import (
	"net/http"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/i18n"
)

// DefaultAuthLayout returns the template name for the built-in auth layout.
// The shipped "auth/layout" is a Tailwind-styled, navbar-less shell that
// links to the host's app/app.min.css (per the Pattern B convention from
// docs/guide/tailwind.md). Hosts that don't follow that convention — or
// want a completely different look — override via [WithAuthLayout]; the
// empty string is also accepted to inherit whatever the host set via
// srv.SetLayout (the pre-v0.20 behaviour, which leaks the host's navbar
// into login pages and is rarely what you want).
func DefaultAuthLayout() string {
	return "auth/layout"
}

// The functions below render auth's built-in pages. Hosts customise a page by
// redefining its template (e.g. {{ define "auth/login" }}) — last-define-wins
// at parse time — so there is no renderer abstraction to swap.

func loginPage(w http.ResponseWriter, r *http.Request, loginRedirect string) error {
	return renderCentered(w, r, i18n.T(r.Context(), "login-title"), "auth/login", map[string]any{
		"LoginRedirect": loginRedirect,
	})
}

func registerPage(w http.ResponseWriter, r *http.Request, useEmail, inviteOnly bool, email, invite string) error {
	return renderCard(w, r, i18n.T(r.Context(), "register-title"), "", "auth/register", map[string]any{
		"UseEmail":   useEmail,
		"InviteOnly": inviteOnly,
		"Email":      email,
		"Invite":     invite,
	})
}

func credentialsPage(w http.ResponseWriter, r *http.Request, creds []Credential) error {
	return renderCard(w, r, i18n.T(r.Context(), "credentials-title"), i18n.T(r.Context(), "credentials-title"), "auth/credentials", map[string]any{
		"Creds": creds,
	})
}

func recoveryPage(w http.ResponseWriter, r *http.Request, loginRedirect string) error {
	return renderCard(w, r, i18n.T(r.Context(), "recovery-title"), "", "auth/recovery", map[string]any{
		"LoginRedirect": loginRedirect,
	})
}

func recoveryCodesPage(w http.ResponseWriter, r *http.Request, codes []string) error {
	return renderCard(w, r, i18n.T(r.Context(), "recovery-codes-title"), i18n.T(r.Context(), "recovery-codes-title"), "auth/recovery_codes", map[string]any{
		"Codes": codes,
	})
}

func verifyPendingPage(w http.ResponseWriter, r *http.Request) error {
	return renderCard(w, r, i18n.T(r.Context(), "verify-pending-title"), i18n.T(r.Context(), "verify-pending-title"), "auth/verify_pending", nil)
}

func verifyEmailSuccessPage(w http.ResponseWriter, r *http.Request) error {
	return renderCard(w, r, i18n.T(r.Context(), "verify-success-title"), i18n.T(r.Context(), "verify-success-title"), "auth/verify_success", nil)
}

func verifyEmailErrorPage(w http.ResponseWriter, r *http.Request, errorCode string) error {
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

	inner, err := exec(r.Context(), name, data)
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

	inner, err := exec(r.Context(), name, data)
	if err != nil {
		return err
	}

	cardData := map[string]any{"Content": inner, "Title": title, "CardTitle": cardTitle}
	return burrow.Render(w, r, http.StatusOK, "auth/card", cardData)
}
