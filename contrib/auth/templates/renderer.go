package templates

import (
	"html/template"
	"maps"
	"net/http"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"codeberg.org/oliverandrich/burrow/contrib/i18n"
)

// AuthLayout returns a minimal HTML layout for unauthenticated auth pages.
// It renders a clean page with Bootstrap CSS but no navbar or navigation.
// Pass this to auth.WithAuthLayout() to override the global app layout
// on login, register, recovery, and verification pages.
func AuthLayout() burrow.LayoutFunc {
	return func(w http.ResponseWriter, r *http.Request, code int, content template.HTML, data map[string]any) error {
		exec := burrow.TemplateExecutorFromContext(r.Context())
		if exec == nil {
			return burrow.HTML(w, code, string(content))
		}

		layoutData := make(map[string]any, len(data)+2)
		maps.Copy(layoutData, data)
		layoutData["Content"] = content
		if _, ok := layoutData["Title"]; !ok {
			layoutData["Title"] = ""
		}

		html, err := exec(r, "auth/layout", layoutData)
		if err != nil {
			return err
		}
		return burrow.HTML(w, code, string(html))
	}
}

// DefaultRenderer returns a Renderer that uses the built-in HTML templates.
// Templates use burrow.RenderTemplate which reads layout from context:
// if a layout is set, page content is wrapped in it; otherwise bare content is rendered.
func DefaultRenderer() auth.Renderer {
	return &defaultRenderer{}
}

// DefaultAdminRenderer returns an AdminRenderer that uses the built-in HTML
// templates for admin pages (users, user detail, invites).
func DefaultAdminRenderer() auth.AdminRenderer {
	return &defaultAdminRenderer{}
}

// defaultRenderer implements auth.Renderer using built-in HTML templates.
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

func (d *defaultRenderer) CredentialsPage(w http.ResponseWriter, r *http.Request, creds []auth.Credential) error {
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

// defaultAdminRenderer implements auth.AdminRenderer using built-in HTML templates.
type defaultAdminRenderer struct{}

func (d *defaultAdminRenderer) AdminUsersPage(w http.ResponseWriter, r *http.Request, users []auth.User) error {
	return burrow.RenderTemplate(w, r, http.StatusOK, "auth/admin_users", map[string]any{
		"Title": i18n.T(r.Context(), "admin-users-title"),
		"Users": users,
	})
}

func (d *defaultAdminRenderer) AdminUserDetailPage(w http.ResponseWriter, r *http.Request, user *auth.User) error {
	return burrow.RenderTemplate(w, r, http.StatusOK, "auth/admin_user_detail", map[string]any{
		"Title": i18n.T(r.Context(), "admin-user-detail-title") + user.Username,
		"User":  user,
	})
}

func (d *defaultAdminRenderer) AdminInvitesPage(w http.ResponseWriter, r *http.Request, invites []auth.Invite, createdURL string, useEmail bool) error {
	return burrow.RenderTemplate(w, r, http.StatusOK, "auth/admin_invites", map[string]any{
		"Title":      i18n.T(r.Context(), "admin-invites-title"),
		"Invites":    invites,
		"CreatedURL": createdURL,
		"UseEmail":   useEmail,
	})
}

// renderCentered renders a template wrapped in the auth/centered layout (no card).
func renderCentered(w http.ResponseWriter, r *http.Request, title, name string, data map[string]any) error {
	exec := burrow.TemplateExecutorFromContext(r.Context())
	if exec == nil {
		return burrow.RenderTemplate(w, r, http.StatusOK, name, data)
	}

	// Render the inner template first.
	inner, err := exec(r, name, data)
	if err != nil {
		return err
	}

	// Wrap in centered container.
	centeredData := map[string]any{"Content": inner, "Title": title}
	return burrow.RenderTemplate(w, r, http.StatusOK, "auth/centered", centeredData)
}

// renderCard renders a template wrapped in the auth/card layout.
func renderCard(w http.ResponseWriter, r *http.Request, title, cardTitle, name string, data map[string]any) error {
	exec := burrow.TemplateExecutorFromContext(r.Context())
	if exec == nil {
		return burrow.RenderTemplate(w, r, http.StatusOK, name, data)
	}

	// Render the inner template first.
	inner, err := exec(r, name, data)
	if err != nil {
		return err
	}

	// Wrap in card. Title is the page title for layout; CardTitle is displayed in the card.
	cardData := map[string]any{"Content": inner, "Title": title, "CardTitle": cardTitle}
	return burrow.RenderTemplate(w, r, http.StatusOK, "auth/card", cardData)
}
