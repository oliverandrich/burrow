package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/oliverandrich/burrow"
	"github.com/go-chi/chi/v5"
)

// CreateInviteRequest is the request body for creating an invite.
type CreateInviteRequest struct {
	Label string `form:"label"`
	Email string `form:"email"`
}

// InvitesPage renders the admin invites management page.
func (h *adminHandlers) InvitesPage(w http.ResponseWriter, r *http.Request) error {
	return h.renderInvitesPage(w, r, "")
}

// CreateInvite creates a new invite and optionally sends an email.
func (h *adminHandlers) CreateInvite(w http.ResponseWriter, r *http.Request) error {
	var req CreateInviteRequest
	if err := burrow.Bind(r, &req); err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	useEmail := h.config != nil && h.config.UseEmail
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
	if err := h.repo.CreateInvite(r.Context(), invite); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to create invite")
	}

	baseURL := ""
	if h.config != nil {
		baseURL = h.config.BaseURL
	}
	createdURL := baseURL + "/auth/register?invite=" + plainToken

	if h.email != nil && req.Email != "" {
		inviteURL := createdURL
		go func() { //nolint:gosec // G118: intentionally detached from request — email must send after response
			sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if sendErr := h.email.SendInvite(sendCtx, req.Email, inviteURL); sendErr != nil {
				slog.Error("failed to send invite email", "error", sendErr, "email", req.Email)
			}
		}()
	}

	slog.Info("invite created", "invite_id", invite.ID, "created_by", user.ID) //nolint:gosec // G706: IDs are int64, not user-controlled strings
	return h.renderInvitesPage(w, r, createdURL)
}

// DeleteInvite revokes an invite by hard-deleting it.
func (h *adminHandlers) DeleteInvite(w http.ResponseWriter, r *http.Request) error {
	inviteID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid invite id")
	}

	if err := h.repo.DeleteInvite(r.Context(), inviteID); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to delete invite")
	}

	w.Header().Set("HX-Redirect", "/admin/invites")
	w.WriteHeader(http.StatusOK)
	return nil
}

func (h *adminHandlers) renderInvitesPage(w http.ResponseWriter, r *http.Request, createdURL string) error {
	invites, err := h.repo.ListInvites(r.Context())
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list invites")
	}
	useEmail := h.config != nil && h.config.UseEmail
	return h.renderer.AdminInvitesPage(w, r, invites, createdURL, useEmail)
}
