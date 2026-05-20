package auth

import (
	"log/slog"
	"net/http"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/session"
)

// redirectTarget reads "redirect_after_login" from the session and validates it,
// falling back to the configured login redirect.
func (a *App[P]) redirectTarget(r *http.Request) string {
	return SafeRedirectPath(session.GetString(r, "redirect_after_login"), a.config.LoginRedirect)
}

func errorJSON(w http.ResponseWriter, statusCode int, msg string) error {
	return burrow.JSON(w, statusCode, map[string]string{"error": msg})
}

func errorJSONLog(w http.ResponseWriter, statusCode int, msg string, err error) error { //nolint:unparam // statusCode is kept for consistency with errorJSON
	if err != nil {
		slog.Error(msg, "error", err)
	}
	return burrow.JSON(w, statusCode, map[string]string{"error": msg})
}
