package auth

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/htmx"
)

// deactivateUserHandler returns a handler that deactivates a user.
func deactivateUserHandler(repo *Repository) burrow.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			return burrow.NewHTTPError(http.StatusBadRequest, "invalid user id")
		}

		if err := repo.SetUserActive(r.Context(), id, false); err != nil {
			return burrow.NewHTTPError(http.StatusInternalServerError, "failed to deactivate user")
		}

		currentUser := UserFromContext(r.Context())
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

// isDeactivatable returns true if the user is active.
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
