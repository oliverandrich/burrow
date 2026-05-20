package auth

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/session"
)

// LoginPage renders the login page.
func (a *App[P]) LoginPage(w http.ResponseWriter, r *http.Request) error {
	return a.renderer.LoginPage(w, r, a.config.LoginRedirect)
}

// LoginBegin starts the WebAuthn discoverable login process.
func (a *App[P]) LoginBegin(w http.ResponseWriter, r *http.Request) error {
	options, sessionData, err := a.webauthn.WebAuthn().BeginDiscoverableLogin()
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to begin login", err)
	}

	sessionID := uuid.New().String()
	a.webauthn.StoreDiscoverableSession(sessionID, sessionData)

	return burrow.JSON(w, http.StatusOK, map[string]any{
		"publicKey":  options.Response,
		"session_id": sessionID,
	})
}

// LoginFinish completes the WebAuthn discoverable login.
func (a *App[P]) LoginFinish(w http.ResponseWriter, r *http.Request) error {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		return errorJSON(w, http.StatusBadRequest, "session_id is required")
	}

	sessionData, err := a.webauthn.GetDiscoverableSession(sessionID)
	if err != nil {
		return errorJSON(w, http.StatusBadRequest, "login session expired")
	}

	var foundUser *User[P]
	credential, finishErr := a.webauthn.WebAuthn().FinishDiscoverableLogin(
		func(rawID, userHandle []byte) (gowebauthn.User, error) {
			userID := string(userHandle)
			if userID == "" {
				return nil, burrow.NewHTTPError(http.StatusBadRequest, "invalid user handle")
			}
			user, userErr := a.repo.GetUserByIDWithCredentials(r.Context(), userID)
			if userErr != nil {
				return nil, userErr
			}
			foundUser = user
			return user, nil
		},
		*sessionData,
		r,
	)
	if finishErr != nil {
		slog.Error("failed to finish discoverable login", "error", finishErr)
		return errorJSON(w, http.StatusUnauthorized, "login failed")
	}

	// Verify sign count to detect cloned credentials before updating.
	if storedCount, ok := findStoredSignCount(foundUser.Credentials, credential.ID); ok {
		if err := verifySignCount(storedCount, credential.Authenticator.SignCount); err != nil {
			slog.Warn("possible cloned credential detected", "user_id", foundUser.ID, "error", err)
			return errorJSON(w, http.StatusForbidden, "credential verification failed")
		}
	}

	if updateErr := a.repo.UpdateCredentialSignCount(r.Context(), credential.ID, credential.Authenticator.SignCount); updateErr != nil {
		slog.Warn("failed to update credential sign count", "error", updateErr)
	}

	if !foundUser.IsActive {
		return errorJSON(w, http.StatusForbidden, "account deactivated")
	}

	if a.UseEmailMode() && a.config.RequireVerification && !foundUser.EmailVerified {
		return burrow.JSON(w, http.StatusForbidden, map[string]any{
			"error":    "email_not_verified",
			"redirect": "/auth/verify-pending",
		})
	}

	// Read redirect target BEFORE session.Save() which replaces all session values.
	redirect := a.redirectTarget(r)

	if err := session.Save(w, r, map[string]any{"user_id": foundUser.ID}); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to create session", err)
	}

	return burrow.JSON(w, http.StatusOK, map[string]string{"status": "ok", "redirect": redirect})
}

// Logout clears the session cookie.
func (a *App[P]) Logout(w http.ResponseWriter, r *http.Request) error {
	session.Clear(w, r)
	htmx.SmartRedirect(w, r, a.config.LogoutRedirect)
	return nil
}

// errSignCountRegressed indicates a possible cloned credential.
var errSignCountRegressed = errors.New("sign count did not increase")

// verifySignCount checks whether the incoming sign count from the authenticator
// is consistent with the stored value.
func verifySignCount(stored, incoming uint32) error {
	if stored == 0 && incoming == 0 {
		return nil
	}
	if incoming > stored {
		return nil
	}
	return fmt.Errorf("%w: stored=%d, incoming=%d", errSignCountRegressed, stored, incoming)
}

// findStoredSignCount looks up the stored sign count for a credential ID
// from the user's preloaded credentials.
func findStoredSignCount(credentials []Credential, credentialID []byte) (uint32, bool) {
	for i := range credentials {
		if bytes.Equal(credentials[i].CredentialID, credentialID) {
			return credentials[i].SignCount, true
		}
	}
	return 0, false
}
