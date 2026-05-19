package auth

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/forms"
)

// adminListUsers handles GET /admin/users — paginated user list with search and role filter.
func (a *App) adminListUsers(w http.ResponseWriter, r *http.Request) error {
	pr := burrow.ParsePageRequest(r)
	role := r.URL.Query().Get("role")
	searchTerm := r.URL.Query().Get("q")

	var (
		users []User
		page  burrow.PageResult
		err   error
	)

	if searchTerm != "" {
		users, page, err = a.repo.SearchUsers(r.Context(), searchTerm, pr, role)
	} else {
		users, page, err = a.repo.ListUsersPaged(r.Context(), pr, role)
	}
	if err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to list users")
	}

	return burrow.Render(w, r, http.StatusOK, "auth/admin_users", map[string]any{
		"Users":      users,
		"Page":       page,
		"Role":       role,
		"SearchTerm": searchTerm,
		"RawQuery":   r.URL.RawQuery,
	})
}

// userFormOpts returns the form options for the user edit form.
func userFormOpts() []forms.Option[User] {
	return []forms.Option[User]{
		forms.WithExclude[User]("ID", "EmailVerifiedAt", "Credentials", "EmailVerified"),
		forms.WithReadOnly[User]("Username", "IsActive", "CreatedAt"),
		forms.WithChoices[User]("Role", []forms.Choice{
			{Value: RoleUser, Label: "User"},
			{Value: RoleAdmin, Label: "Administrator"},
		}),
	}
}

// adminEditUser handles GET /admin/users/{id} — user edit form.
func (a *App) adminEditUser(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")

	user, err := a.repo.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return burrow.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to get user")
	}

	f := forms.FromModel(user, userFormOpts()...)
	return burrow.Render(w, r, http.StatusOK, "auth/admin_user_form", map[string]any{
		"User":   user,
		"Fields": f.Fields(),
	})
}

// adminUpdateUser handles POST /admin/users/{id} — update user.
func (a *App) adminUpdateUser(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")

	user, err := a.repo.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return burrow.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to get user")
	}

	f := forms.FromModel(user, userFormOpts()...)
	if !f.Bind(r) {
		return burrow.Render(w, r, http.StatusOK, "auth/admin_user_form", map[string]any{
			"User":           user,
			"Fields":         f.Fields(),
			"NonFieldErrors": f.NonFieldErrors(),
		})
	}

	updated := f.Instance()
	updated.ID = user.ID
	updated.Username = user.Username
	updated.IsActive = user.IsActive

	// Prevent demoting the last admin.
	if updated.Role != RoleAdmin && user.Role == RoleAdmin {
		adminCount, err := a.repo.CountAdminUsers(r.Context())
		if err != nil {
			return burrow.NewHTTPError(http.StatusInternalServerError, "failed to count admins")
		}
		if adminCount <= 1 {
			return burrow.NewHTTPError(http.StatusUnprocessableEntity, "cannot demote the last admin")
		}
	}

	if err := a.repo.UpdateUser(r.Context(), updated); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to update user")
	}

	// Support "save & continue editing".
	if r.FormValue("_continue") == "1" { //nolint:gosec // _continue is a UI flag, not user input
		htmx.SmartRedirect(w, r, "/admin/users/"+user.ID)
		return nil
	}

	htmx.SmartRedirect(w, r, "/admin/users")
	return nil
}

// adminDeleteUser handles DELETE /admin/users/{id} — delete user.
func (a *App) adminDeleteUser(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	if id == "" {
		return burrow.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	if err := a.repo.DeleteUser(r.Context(), id); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to delete user")
	}

	currentUser := CurrentUser(r.Context())
	slog.Info("user deleted", "user_id", id, "deleted_by", currentUser.ID) //nolint:gosec
	htmx.SmartRedirect(w, r, "/admin/users")
	return nil
}

// deactivateUserHandler returns a handler that deactivates a user.
func deactivateUserHandler(repo *Repository) burrow.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := chi.URLParam(r, "id")
		if id == "" {
			return burrow.NewHTTPError(http.StatusBadRequest, "invalid user id")
		}

		if err := repo.SetUserActive(r.Context(), id, false); err != nil {
			return burrow.NewHTTPError(http.StatusInternalServerError, "failed to deactivate user")
		}

		currentUser := CurrentUser(r.Context())
		slog.Info("user deactivated", "user_id", id, "deactivated_by", currentUser.ID) //nolint:gosec // user IDs are ULIDs, not user input
		htmx.SmartRedirect(w, r, "/admin/users")
		return nil
	}
}

// activateUserHandler returns a handler that activates a user.
func activateUserHandler(repo *Repository) burrow.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := chi.URLParam(r, "id")
		if id == "" {
			return burrow.NewHTTPError(http.StatusBadRequest, "invalid user id")
		}

		if err := repo.SetUserActive(r.Context(), id, true); err != nil {
			return burrow.NewHTTPError(http.StatusInternalServerError, "failed to activate user")
		}

		currentUser := CurrentUser(r.Context())
		slog.Info("user activated", "user_id", id, "activated_by", currentUser.ID) //nolint:gosec // user IDs are ULIDs, not user input
		htmx.SmartRedirect(w, r, "/admin/users")
		return nil
	}
}
