package auth

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/oliverandrich/burrow/i18n"
)

// CreateInviteRequest is the request body for creating an invite.
type CreateInviteRequest struct {
	Label string `form:"label"`
	Email string `form:"email"`
}

// adminListInvites handles GET /admin/invites — paginated invite list.
func (a *App[P]) adminListInvites(w http.ResponseWriter, r *http.Request) error {
	pr := burrow.ParsePageRequest(r)
	searchTerm := r.URL.Query().Get("q")

	var (
		invites []Invite
		page    burrow.PageResult
		err     error
	)
	if searchTerm != "" {
		invites, page, err = a.repo.SearchInvitesPaged(r.Context(), searchTerm, pr)
	} else {
		invites, page, err = a.repo.ListInvitesPaged(r.Context(), pr)
	}
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list invites")
	}

	// One-shot pickup of an invite URL stashed by handleCreateInvite for the
	// no-email path. Cleared immediately so a refresh doesn't re-show it.
	var createdURL string
	if values := session.GetValues(r); values != nil {
		if raw, ok := values[sessionKeyInviteCreatedURL]; ok {
			if u, ok := raw.(string); ok && u != "" {
				createdURL = u
				_ = session.Delete(w, r, sessionKeyInviteCreatedURL)
			}
		}
	}

	return burrow.Render(w, r, http.StatusOK, "auth/admin_invites", map[string]any{
		"Invites":    invites,
		"Page":       page,
		"SearchTerm": searchTerm,
		"UseEmail":   a.config != nil && a.config.UseEmail,
		"RawQuery":   r.URL.RawQuery,
		"CreatedURL": createdURL,
	})
}

// adminNewInviteForm handles GET /admin/invites/new — returns the invite form fragment for htmx.
func (a *App[P]) adminNewInviteForm(w http.ResponseWriter, r *http.Request) error {
	return burrow.Render(w, r, http.StatusOK, "auth/admin_invite_form", map[string]any{
		"UseEmail": a.config != nil && a.config.UseEmail,
	})
}

// handleCreateInvite creates a new invite and optionally sends an email.
func (a *App[P]) handleCreateInvite(w http.ResponseWriter, r *http.Request) error {
	var req CreateInviteRequest
	if err := burrow.Bind(r, &req); err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	useEmail := a.config != nil && a.config.UseEmail
	if useEmail && req.Email == "" {
		return burrow.NewHTTPError(http.StatusBadRequest, "email is required")
	}

	user := CurrentUser[P](r.Context())
	if user == nil {
		return burrow.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	plainToken, tokenHash, err := GenerateInviteToken()
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to generate invite token")
	}

	invite := &Invite{
		Email:     req.Email,
		Label:     req.Label,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(InviteExpiry),
		CreatedBy: &user.ID,
	}
	if err := a.repo.CreateInvite(r.Context(), invite); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to create invite")
	}

	baseURL := ""
	if a.config != nil {
		baseURL = a.config.BaseURL
	}
	createdURL := baseURL + "/auth/register?invite=" + plainToken

	if a.emailService != nil && req.Email != "" {
		if err := a.enqueueEmail(r.Context(), "invite", req.Email, createdURL); err != nil {
			slog.Error("failed to enqueue invite email", "error", err, "email", req.Email)
		}
		if err := messages.AddSuccess(w, r, i18n.T(r.Context(), "admin-invites-sent")); err != nil {
			slog.Warn("failed to add invite flash message", "error", err)
		}
	} else {
		// No email sent — stash the URL in the session so the list page can
		// render it in a copyable input + button next to the alert.
		if err := session.Set(w, r, sessionKeyInviteCreatedURL, createdURL); err != nil {
			slog.Warn("failed to store invite URL in session", "error", err)
		}
	}

	slog.Info("invite created", "invite_id", invite.ID, "created_by", user.ID)
	htmx.SmartRedirect(w, r, "/admin/invites")
	return nil
}

// sessionKeyInviteCreatedURL holds a freshly-created invite URL passed
// through the session so [adminListInvites] can show a copyable URL on
// the next page render. Cleared after one use.
const sessionKeyInviteCreatedURL = "admin-invite-created-url"

// revokeInviteHandler returns a handler that revokes (hard-deletes) an invite.
func revokeInviteHandler[P any](repo *Repository[P]) burrow.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		inviteID := chi.URLParam(r, "id")
		if inviteID == "" {
			return burrow.NewHTTPError(http.StatusBadRequest, "invalid invite id")
		}

		if err := repo.DeleteInvite(r.Context(), inviteID); err != nil {
			return burrow.NewHTTPError(http.StatusInternalServerError, "failed to delete invite")
		}

		htmx.SmartRedirect(w, r, "/admin/invites")
		return nil
	}
}
