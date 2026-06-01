package auth

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
)

// CredentialsPage renders the credentials management page.
func (a *App[P]) CredentialsPage(w http.ResponseWriter, r *http.Request) error {
	user := MustCurrentUser[P](r.Context())
	creds, err := a.repo.GetCredentialsByUserID(r.Context(), user.ID)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to get credentials", err)
	}
	return credentialsPage(w, r, creds)
}

// AddCredentialBegin starts the process of adding a new credential.
func (a *App[P]) AddCredentialBegin(w http.ResponseWriter, r *http.Request) error {
	user := MustCurrentUser[P](r.Context())
	options, sessionData, err := a.webauthn.WebAuthn().BeginRegistration(user)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to begin registration", err)
	}
	a.webauthn.StoreRegistrationSession(user.ID, sessionData)

	return burrow.JSON(w, http.StatusOK, map[string]any{"publicKey": options.Response})
}

// AddCredentialFinish completes adding a new credential.
func (a *App[P]) AddCredentialFinish(w http.ResponseWriter, r *http.Request) error {
	user := MustCurrentUser[P](r.Context())
	sessionData, err := a.webauthn.GetRegistrationSession(user.ID)
	if err != nil {
		return errorJSON(w, http.StatusBadRequest, "registration session expired")
	}

	credential, err := a.webauthn.WebAuthn().FinishRegistration(user, *sessionData, r)
	if err != nil {
		slog.Error("registration failed", "error", err)
		return errorJSON(w, http.StatusBadRequest, "registration failed")
	}

	dbCred := NewCredentialFromWebAuthn(user.ID, credential)
	if err := a.repo.CreateCredential(r.Context(), dbCred); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to store credential", err)
	}

	return burrow.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteCredential removes a credential.
func (a *App[P]) DeleteCredential(w http.ResponseWriter, r *http.Request) error {
	user := MustCurrentUser[P](r.Context())
	credID := chi.URLParam(r, "id")
	if credID == "" {
		return errorJSON(w, http.StatusBadRequest, "invalid credential id")
	}

	count, err := a.repo.CountUserCredentials(r.Context(), user.ID)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "database error", err)
	}
	if count <= 1 {
		return errorJSON(w, http.StatusBadRequest, "cannot delete last credential")
	}

	if err := a.repo.DeleteCredential(r.Context(), credID, user.ID); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to delete credential", err)
	}

	return burrow.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
