package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/oliverandrich/burrow"
	"github.com/go-chi/chi/v5"
)

// AdminRenderer defines how admin user pages are rendered.
// Projects implement this to provide their own template rendering
// for the admin area.
type AdminRenderer interface {
	AdminUsersPage(w http.ResponseWriter, r *http.Request, users []User) error
	AdminUserDetailPage(w http.ResponseWriter, r *http.Request, user *User) error
	AdminInvitesPage(w http.ResponseWriter, r *http.Request, invites []Invite, createdURL string, useEmail bool) error
}

// adminHandlers holds the admin HTTP handlers for user management.
type adminHandlers struct {
	repo     *Repository
	renderer AdminRenderer
	config   *Config
	email    EmailService
}

// newAdminHandlers creates admin handlers with the given repo and renderer.
func newAdminHandlers(repo *Repository, renderer AdminRenderer, config *Config, email EmailService) *adminHandlers {
	return &adminHandlers{repo: repo, renderer: renderer, config: config, email: email}
}

// UsersPage renders the admin user list.
func (h *adminHandlers) UsersPage(w http.ResponseWriter, r *http.Request) error {
	users, err := h.repo.ListUsers(r.Context())
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list users")
	}
	return h.renderer.AdminUsersPage(w, r, users)
}

// UserDetail renders the admin user detail page.
func (h *adminHandlers) UserDetail(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	user, err := h.repo.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return burrow.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to get user")
	}

	return h.renderer.AdminUserDetailPage(w, r, user)
}

// CreateInviteRequest is the request body for creating an invite.
type CreateInviteRequest struct {
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
		return errorJSON(w, http.StatusBadRequest, "invalid request")
	}

	useEmail := h.config != nil && h.config.UseEmail
	if useEmail && req.Email == "" {
		return errorJSON(w, http.StatusBadRequest, "email is required")
	}

	user := GetUser(r)
	if user == nil {
		return errorJSON(w, http.StatusUnauthorized, "unauthorized")
	}

	plainToken, tokenHash, err := GenerateInviteToken()
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to generate invite token", err)
	}

	invite := &Invite{
		Email:     req.Email,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(InviteExpiry),
		CreatedBy: &user.ID,
	}
	if err := h.repo.CreateInvite(r.Context(), invite); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to create invite", err)
	}

	if h.email != nil && req.Email != "" {
		go func() {
			sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if sendErr := h.email.SendInvite(sendCtx, req.Email, plainToken); sendErr != nil {
				slog.Error("failed to send invite email", "error", sendErr, "email", req.Email)
			}
		}()
	}

	slog.Info("invite created", "invite_id", invite.ID, "created_by", user.ID) //nolint:gosec // G706: IDs are int64, not user-controlled strings

	baseURL := ""
	if h.config != nil {
		baseURL = h.config.BaseURL
	}
	createdURL := baseURL + "/auth/register?invite=" + plainToken
	return h.renderInvitesPage(w, r, createdURL)
}

// DeleteInvite revokes an invite by hard-deleting it.
func (h *adminHandlers) DeleteInvite(w http.ResponseWriter, r *http.Request) error {
	inviteID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errorJSON(w, http.StatusBadRequest, "invalid invite id")
	}

	if err := h.repo.DeleteInvite(r.Context(), inviteID); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to delete invite", err)
	}

	return burrow.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *adminHandlers) renderInvitesPage(w http.ResponseWriter, r *http.Request, createdURL string) error {
	invites, err := h.repo.ListInvites(r.Context())
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to list invites", err)
	}
	useEmail := h.config != nil && h.config.UseEmail
	return h.renderer.AdminInvitesPage(w, r, invites, createdURL, useEmail)
}

// UpdateUserRole changes a user's role.
func (h *adminHandlers) UpdateUserRole(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	role := r.FormValue("role")
	if role != RoleAdmin && role != RoleUser {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid role")
	}

	// Verify user exists.
	if _, err := h.repo.GetUserByID(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return burrow.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to get user")
	}

	if err := h.repo.SetUserRole(r.Context(), id, role); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to update role")
	}

	http.Redirect(w, r, "/admin/users/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	return nil
}
