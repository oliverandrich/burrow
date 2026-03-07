package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

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

	currentUser := UserFromContext(r.Context())
	isSelf := currentUser != nil && currentUser.ID == user.ID

	adminCount, err := h.repo.CountAdminUsers(r.Context())
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to count admins")
	}
	isLastAdmin := user.Role == RoleAdmin && adminCount <= 1

	ctx := withAdminEditFlags(r.Context(), isSelf, isLastAdmin)
	return h.renderer.AdminUserDetailPage(w, r.WithContext(ctx), user)
}

// DeleteUser soft-deletes a user by ID.
func (h *adminHandlers) DeleteUser(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	// Prevent self-deletion.
	currentUser := UserFromContext(r.Context())
	if currentUser != nil && currentUser.ID == id {
		return burrow.NewHTTPError(http.StatusBadRequest, "cannot delete your own account")
	}

	// Verify user exists.
	if _, err := h.repo.GetUserByID(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return burrow.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to get user")
	}

	if err := h.repo.DeleteUser(r.Context(), id); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to delete user")
	}

	slog.Info("user deleted", "user_id", id, "deleted_by", currentUser.ID) //nolint:gosec // G706: IDs are int64, not user-controlled strings
	w.Header().Set("HX-Redirect", "/admin/users")
	w.WriteHeader(http.StatusOK)
	return nil
}

// UpdateUser updates a user's editable fields (name, bio, email, role).
func (h *adminHandlers) UpdateUser(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	role := r.FormValue("role") //nolint:gosec // G120: body size limited by server-level RequestSize middleware
	if role != RoleAdmin && role != RoleUser {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid role")
	}

	user, err := h.repo.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return burrow.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to get user")
	}

	// Prevent demotion of the last admin.
	if user.Role == RoleAdmin && role != RoleAdmin {
		adminCount, err := h.repo.CountAdminUsers(r.Context())
		if err != nil {
			return burrow.NewHTTPError(http.StatusInternalServerError, "failed to count admins")
		}
		if adminCount <= 1 {
			return burrow.NewHTTPError(http.StatusBadRequest, "cannot remove the last admin")
		}
	}

	user.Name = r.FormValue("name") //nolint:gosec // G120: body size limited by server-level RequestSize middleware
	user.Bio = r.FormValue("bio")   //nolint:gosec // G120: body size limited by server-level RequestSize middleware
	user.Role = role

	email := r.FormValue("email") //nolint:gosec // G120: body size limited by server-level RequestSize middleware
	if email != "" {
		user.Email = &email
	} else {
		user.Email = nil
	}

	if err := h.repo.UpdateUser(r.Context(), user); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to update user")
	}

	redirectURL := "/admin/users"
	if r.FormValue("_continue") != "" { //nolint:gosec // G120: body size limited by server-level RequestSize middleware
		redirectURL = "/admin/users/" + strconv.FormatInt(id, 10)
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	return nil
}
