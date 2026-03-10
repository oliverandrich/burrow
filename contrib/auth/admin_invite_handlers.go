package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/i18n"
	"github.com/oliverandrich/burrow/contrib/messages"
)

// CreateInviteRequest is the request body for creating an invite.
type CreateInviteRequest struct {
	Label string `form:"label"`
	Email string `form:"email"`
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

	user := UserFromContext(r.Context())
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
		inviteURL := createdURL
		locale := i18n.Locale(r.Context())
		go func() { //nolint:gosec // G118: intentionally detached from request — email must send after response
			sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			sendCtx = a.i18nApp.WithLocale(sendCtx, locale)
			if sendErr := a.emailService.SendInvite(sendCtx, req.Email, inviteURL); sendErr != nil {
				slog.Error("failed to send invite email", "error", sendErr, "email", req.Email)
			}
		}()
		flashMsg = i18n.T(r.Context(), "admin-invites-sent")
	} else {
		flashMsg = i18n.T(r.Context(), "admin-invites-copy-url") + " " + createdURL
	}

	if err := messages.AddSuccess(w, r, flashMsg); err != nil {
		slog.Warn("failed to add invite flash message", "error", err)
	}

	slog.Info("invite created", "invite_id", invite.ID, "created_by", user.ID) //nolint:gosec // G706: IDs are int64, not user-controlled strings
	http.Redirect(w, r, "/admin/invites", http.StatusSeeOther)
	return nil
}

// revokeInviteHandler returns a handler that revokes (hard-deletes) an invite.
func revokeInviteHandler(repo *Repository) burrow.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		inviteID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			return burrow.NewHTTPError(http.StatusBadRequest, "invalid invite id")
		}

		if err := repo.DeleteInvite(r.Context(), inviteID); err != nil {
			return burrow.NewHTTPError(http.StatusInternalServerError, "failed to delete invite")
		}

		htmx.Redirect(w, "/admin/invites")
		w.WriteHeader(http.StatusOK)
		return nil
	}
}

// isRevokable returns true if the invite is active (not used and not expired).
func isRevokable(item any) bool {
	inv, ok := item.(Invite)
	if !ok {
		return false
	}
	return !inv.IsUsed() && !inv.IsExpired()
}
