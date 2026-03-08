package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/htmx"
	"github.com/go-chi/chi/v5"
)

// handleUserDetail renders the admin user detail page with edit flags.
func (a *App) handleUserDetail(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	user, err := a.repo.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return burrow.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to get user")
	}

	currentUser := UserFromContext(r.Context())
	isSelf := currentUser != nil && currentUser.ID == user.ID

	adminCount, err := a.repo.CountAdminUsers(r.Context())
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to count admins")
	}
	isLastAdmin := user.Role == RoleAdmin && adminCount <= 1

	ctx := withAdminEditFlags(r.Context(), isSelf, isLastAdmin)
	return burrow.RenderTemplate(w, r.WithContext(ctx), http.StatusOK, "auth/admin_user_detail", map[string]any{
		"Title": user.Username,
		"User":  user,
	})
}

// handleUpdateUser updates a user's editable fields (name, bio, email, role).
func (a *App) handleUpdateUser(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	role := r.FormValue("role") //nolint:gosec // G120: body size limited by server-level RequestSize middleware
	if role != RoleAdmin && role != RoleUser {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid role")
	}

	user, err := a.repo.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return burrow.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to get user")
	}

	// Prevent demotion of the last admin.
	if user.Role == RoleAdmin && role != RoleAdmin {
		adminCount, err := a.repo.CountAdminUsers(r.Context())
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

	if err := a.repo.UpdateUser(r.Context(), user); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to update user")
	}

	redirectURL := "/admin/users"
	if r.FormValue("_continue") != "" { //nolint:gosec // G120: body size limited by server-level RequestSize middleware
		redirectURL = "/admin/users/" + strconv.FormatInt(id, 10)
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	return nil
}

// deactivateUserHandler returns a handler that deactivates a user.
func deactivateUserHandler(repo *Repository) burrow.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			return burrow.NewHTTPError(http.StatusBadRequest, "invalid user id")
		}

		currentUser := UserFromContext(r.Context())
		if currentUser != nil && currentUser.ID == id {
			return burrow.NewHTTPError(http.StatusBadRequest, "cannot deactivate your own account")
		}

		if err := repo.SetUserActive(r.Context(), id, false); err != nil {
			return burrow.NewHTTPError(http.StatusInternalServerError, "failed to deactivate user")
		}

		slog.Info("user deactivated", "user_id", id, "deactivated_by", currentUser.ID) //nolint:gosec // G706: IDs are int64
		htmx.Redirect(w, "/admin/users")
		w.WriteHeader(http.StatusOK)
		return nil
	}
}

// activateUserHandler returns a handler that activates a user.
func activateUserHandler(repo *Repository) burrow.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			return burrow.NewHTTPError(http.StatusBadRequest, "invalid user id")
		}

		if err := repo.SetUserActive(r.Context(), id, true); err != nil {
			return burrow.NewHTTPError(http.StatusInternalServerError, "failed to activate user")
		}

		currentUser := UserFromContext(r.Context())
		slog.Info("user activated", "user_id", id, "activated_by", currentUser.ID) //nolint:gosec // G706: IDs are int64
		htmx.Redirect(w, "/admin/users")
		w.WriteHeader(http.StatusOK)
		return nil
	}
}

// isDeactivatable returns true if the user is active and not the current request user.
// Since ShowWhen has no access to the request, we only check IsActive here.
// The handler itself prevents self-deactivation.
func isDeactivatable(item any) bool {
	u, ok := item.(User)
	if !ok {
		return false
	}
	return u.IsActive
}

// isActivatable returns true if the user is inactive.
func isActivatable(item any) bool {
	u, ok := item.(User)
	if !ok {
		return false
	}
	return !u.IsActive
}

// handleDeleteUser soft-deletes a user by ID.
func (a *App) handleDeleteUser(w http.ResponseWriter, r *http.Request) error {
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
	user, err := a.repo.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return burrow.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to get user")
	}

	// Prevent deletion of the last admin.
	if user.Role == RoleAdmin {
		adminCount, err := a.repo.CountAdminUsers(r.Context())
		if err != nil {
			return burrow.NewHTTPError(http.StatusInternalServerError, "failed to count admins")
		}
		if adminCount <= 1 {
			return burrow.NewHTTPError(http.StatusBadRequest, "cannot delete the last admin")
		}
	}

	if err := a.repo.DeleteUser(r.Context(), id); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to delete user")
	}

	slog.Info("user deleted", "user_id", id, "deleted_by", currentUser.ID) //nolint:gosec // G706: IDs are int64, not user-controlled strings
	htmx.Redirect(w, "/admin/users")
	w.WriteHeader(http.StatusOK)
	return nil
}
