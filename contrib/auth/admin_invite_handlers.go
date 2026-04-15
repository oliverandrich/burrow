package auth

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/burrow/i18n"
)

// CreateInviteRequest is the request body for creating an invite.
type CreateInviteRequest struct {
	Label string `form:"label"`
	Email string `form:"email"`
}

// adminListInvites handles GET /admin/invites — paginated invite list.
func (a *App) adminListInvites(w http.ResponseWriter, r *http.Request) error {
	pr := burrow.ParsePageRequest(r)

	invites, page, err := a.repo.ListInvitesPaged(r.Context(), pr)
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list invites")
	}

	return burrow.Render(w, r, http.StatusOK, "auth/admin_invites", map[string]any{
		"Invites":  invites,
		"Page":     page,
		"UseEmail": a.config != nil && a.config.UseEmail,
	})
}

// adminNewInviteForm handles GET /admin/invites/new — returns the invite form fragment for htmx.
func (a *App) adminNewInviteForm(w http.ResponseWriter, r *http.Request) error {
	return burrow.Render(w, r, http.StatusOK, "auth/admin_invite_form", map[string]any{
		"UseEmail": a.config != nil && a.config.UseEmail,
	})
}

// handleCreateInvite creates a new invite and optionally sends an email.
func (a *App) handleCreateInvite(w http.ResponseWriter, r *http.Request) error {
	var req CreateInviteRequest
	if err := burrow.Bind(r, &req); err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	useEmail := a.config != nil && a.config.UseEmail
	if useEmail && req.Email == "" {
		return burrow.NewHTTPError(http.StatusBadRequest, "email is required")
	}

	user := CurrentUser(r.Context())
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

	var flashMsg string
	if a.emailService != nil && req.Email != "" {
		if err := a.enqueueEmail(r.Context(), "invite", req.Email, createdURL); err != nil {
			slog.Error("failed to enqueue invite email", "error", err, "email", req.Email)
		}
		flashMsg = i18n.T(r.Context(), "admin-invites-sent")
	} else {
		flashMsg = i18n.T(r.Context(), "admin-invites-copy-url") + " " + createdURL
	}

	if err := messages.AddSuccess(w, r, flashMsg); err != nil {
		slog.Warn("failed to add invite flash message", "error", err)
	}

	slog.Info("invite created", "invite_id", invite.ID, "created_by", user.ID)
	htmx.SmartRedirect(w, r, "/admin/invites")
	return nil
}

// revokeInviteHandler returns a handler that revokes (hard-deletes) an invite.
func revokeInviteHandler(repo *Repository) burrow.HandlerFunc {
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
