package admin

import (
	"errors"
	"net/http"
	"strconv"

	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"github.com/labstack/echo/v5"
)

// Renderer defines how admin pages are rendered.
type Renderer interface {
	UsersPage(c *echo.Context, users []auth.User) error
	UserDetailPage(c *echo.Context, user *auth.User) error
}

// Handlers holds the admin HTTP handlers.
type Handlers struct {
	store    Store
	renderer Renderer
}

// NewHandlers creates admin handlers with the given store and renderer.
func NewHandlers(store Store, renderer Renderer) *Handlers {
	return &Handlers{store: store, renderer: renderer}
}

// UsersPage renders the user list.
func (h *Handlers) UsersPage(c *echo.Context) error {
	users, err := h.store.ListUsers(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list users")
	}
	return h.renderer.UsersPage(c, users)
}

// UserDetail renders the user detail page.
func (h *Handlers) UserDetail(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	user, err := h.store.GetUserByID(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user")
	}

	return h.renderer.UserDetailPage(c, user)
}

// UpdateUserRole changes a user's role.
func (h *Handlers) UpdateUserRole(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	role := c.FormValue("role")
	if role != auth.RoleAdmin && role != auth.RoleUser {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid role")
	}

	// Verify user exists.
	if _, err := h.store.GetUserByID(c.Request().Context(), id); err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user")
	}

	if err := h.store.SetUserRole(c.Request().Context(), id, role); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update role")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/users/"+strconv.FormatInt(id, 10))
}
