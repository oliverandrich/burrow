package auth

import (
	"errors"
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
}

// adminHandlers holds the admin HTTP handlers for user management.
type adminHandlers struct {
	repo     *Repository
	renderer AdminRenderer
}

// newAdminHandlers creates admin handlers with the given repo and renderer.
func newAdminHandlers(repo *Repository, renderer AdminRenderer) *adminHandlers {
	return &adminHandlers{repo: repo, renderer: renderer}
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
