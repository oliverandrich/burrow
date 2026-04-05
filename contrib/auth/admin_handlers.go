package auth

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/htmx"
)

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
