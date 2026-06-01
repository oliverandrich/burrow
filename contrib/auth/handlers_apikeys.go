package auth

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/session"
)

// sessionKeyNewAPIKey holds a freshly-created plaintext API key for exactly
// one render: CreateAPIKeyHandler stashes it, APIKeysPage shows it once and
// clears it. The plaintext is never stored anywhere else.
const sessionKeyNewAPIKey = "new_api_key"

// APIKeysPage renders the user's API keys and the create form. If a key was
// just created, its plaintext is shown once (then cleared from the session).
func (a *App[P]) APIKeysPage(w http.ResponseWriter, r *http.Request) error {
	user := MustCurrentUser[P](r.Context())
	keys, err := a.repo.ListAPIKeysByUser(r.Context(), user.ID)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to list api keys", err)
	}

	newKey := session.GetString(r, sessionKeyNewAPIKey)
	if newKey != "" {
		if delErr := session.Delete(w, r, sessionKeyNewAPIKey); delErr != nil {
			return errorJSONLog(w, http.StatusInternalServerError, "failed to clear new api key", delErr)
		}
	}

	return apiKeysPage(w, r, keys, newKey)
}

// CreateAPIKeyHandler mints a key for the current user, stashes its plaintext
// in the session for one render, and redirects back to the list.
func (a *App[P]) CreateAPIKeyHandler(w http.ResponseWriter, r *http.Request) error {
	user := MustCurrentUser[P](r.Context())
	label := strings.TrimSpace(r.FormValue("label"))

	plaintext, _, err := a.repo.CreateAPIKey(r.Context(), user.ID, label, nil)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to create api key", err)
	}
	if err := session.Set(w, r, sessionKeyNewAPIKey, plaintext); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to stash new api key", err)
	}

	htmx.SmartRedirect(w, r, "/auth/api-keys")
	return nil
}

// RevokeAPIKeyHandler deletes one of the current user's keys (scoped to the
// owner, so the path id can't reach another user's key) and redirects back.
func (a *App[P]) RevokeAPIKeyHandler(w http.ResponseWriter, r *http.Request) error {
	user := MustCurrentUser[P](r.Context())
	id := chi.URLParam(r, "id")
	if id == "" {
		return errorJSON(w, http.StatusBadRequest, "invalid api key id")
	}

	if err := a.repo.DeleteAPIKey(r.Context(), id, user.ID); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to revoke api key", err)
	}

	htmx.SmartRedirect(w, r, "/auth/api-keys")
	return nil
}
