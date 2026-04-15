package auth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/session"
	"golang.org/x/crypto/bcrypt"
)

// Renderer defines the page rendering interface for auth templates.
// Projects implement this to provide their own template rendering.
type Renderer interface {
	RegisterPage(w http.ResponseWriter, r *http.Request, useEmail, inviteOnly bool, email, invite string) error
	LoginPage(w http.ResponseWriter, r *http.Request, loginRedirect string) error
	CredentialsPage(w http.ResponseWriter, r *http.Request, creds []Credential) error
	RecoveryPage(w http.ResponseWriter, r *http.Request, loginRedirect string) error
	RecoveryCodesPage(w http.ResponseWriter, r *http.Request, codes []string) error
	VerifyPendingPage(w http.ResponseWriter, r *http.Request) error
	VerifyEmailSuccessPage(w http.ResponseWriter, r *http.Request) error
	VerifyEmailErrorPage(w http.ResponseWriter, r *http.Request, errorCode string) error
}

// UseEmailMode returns true if email-based authentication is enabled.
func (a *App) UseEmailMode() bool {
	return a.config != nil && a.config.UseEmail
}

// IsInviteOnly returns true if invite-only registration is enabled.
func (a *App) IsInviteOnly() bool {
	return a.config != nil && a.config.InviteOnly
}

// --- Registration ---

// RegisterBeginRequest is the request body for starting registration.
type RegisterBeginRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Invite   string `json:"invite"`
}

// RegisterPage renders the registration page.
func (a *App) RegisterPage(w http.ResponseWriter, r *http.Request) error {
	inviteToken := r.URL.Query().Get("invite")

	if a.IsInviteOnly() && inviteToken != "" {
		invite, err := a.validateInviteToken(r.Context(), inviteToken)
		if err != nil || !invite.IsValid() {
			return a.renderer.RegisterPage(w, r, a.UseEmailMode(), true, "", "")
		}
		return a.renderer.RegisterPage(w, r, a.UseEmailMode(), true, invite.Email, inviteToken)
	}

	return a.renderer.RegisterPage(w, r, a.UseEmailMode(), a.IsInviteOnly(), "", "")
}

// RegisterBegin starts the WebAuthn registration process.
func (a *App) RegisterBegin(w http.ResponseWriter, r *http.Request) error {
	var req RegisterBeginRequest
	if err := burrow.Bind(r, &req); err != nil {
		return errorJSON(w, http.StatusBadRequest, "invalid request")
	}

	ctx := r.Context()

	// Invite-only mode: validate invite token (first user bypasses).
	var validInvite *Invite
	if a.IsInviteOnly() {
		isFirst, err := a.isFirstUser(ctx)
		if err != nil {
			return errorJSONLog(w, http.StatusInternalServerError, "database error", err)
		}
		if !isFirst {
			if req.Invite == "" {
				return errorJSON(w, http.StatusForbidden, "invite token required")
			}
			invite, validateErr := a.validateInviteToken(ctx, req.Invite)
			if validateErr != nil || !invite.IsValid() {
				return errorJSON(w, http.StatusForbidden, "invalid or expired invite")
			}
			validInvite = invite

			if a.UseEmailMode() && req.Email != invite.Email {
				return errorJSON(w, http.StatusForbidden, "email does not match invite")
			}
		}
	}

	var user *User
	var createErr error

	if a.UseEmailMode() {
		if req.Email == "" {
			return errorJSON(w, http.StatusBadRequest, "email is required")
		}
		user, createErr = a.repo.CreateUserWithEmail(ctx, req.Email, req.Name)
	} else {
		if req.Username == "" {
			return errorJSON(w, http.StatusBadRequest, "username is required")
		}
		user, createErr = a.repo.CreateUser(ctx, req.Username, req.Name)
	}

	if createErr != nil {
		// UNIQUE constraint violation means the username/email is already taken.
		// Return a generic message without revealing which field conflicted.
		// This also eliminates the TOCTOU race between existence check and insert.
		return errorJSON(w, http.StatusOK, "registration failed")
	}

	// Clean up the user if any subsequent step fails, so abandoned
	// registrations don't permanently block the username/email.
	registered := false
	defer func() {
		if !registered {
			if delErr := a.repo.DeleteUser(ctx, user.ID); delErr != nil {
				slog.Error("failed to clean up orphaned user", "user_id", user.ID, "error", delErr)
			}
		}
	}()

	// Promote to admin if no admin exists yet. Using CountAdminUsers
	// instead of CountUsers avoids a race with phantom users from
	// abandoned registration flows.
	adminCount, countErr := a.repo.CountAdminUsers(ctx)
	if countErr == nil && adminCount == 0 {
		if roleErr := a.repo.SetUserRole(ctx, user.ID, RoleAdmin); roleErr != nil {
			slog.Error("failed to promote first user to admin", "user_id", user.ID, "error", roleErr)
		}
		user.Role = RoleAdmin
		slog.Info("first user registered as admin", "user_id", user.ID)
	}

	// Mark invite as used. If the invite was already consumed by a concurrent
	// request, abort registration and let the cleanup defer delete the user.
	if validInvite != nil {
		if markErr := a.repo.MarkInviteUsed(ctx, validInvite.ID, user.ID); markErr != nil {
			slog.Error("failed to mark invite as used", "invite_id", validInvite.ID, "error", markErr)
			return errorJSON(w, http.StatusConflict, "registration failed")
		}
	}

	// Begin WebAuthn registration.
	options, sessionData, err := a.webauthn.WebAuthn().BeginRegistration(user)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to begin registration", err)
	}
	a.webauthn.StoreRegistrationSession(user.ID, sessionData)

	registered = true
	return burrow.JSON(w, http.StatusOK, map[string]any{
		"publicKey": options.Response,
		"user_id":   user.ID,
	})
}

// RegisterFinish completes the WebAuthn registration process.
func (a *App) RegisterFinish(w http.ResponseWriter, r *http.Request) error {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		return errorJSON(w, http.StatusBadRequest, "invalid user_id")
	}

	ctx := r.Context()

	sessionData, err := a.webauthn.GetRegistrationSession(userID)
	if err != nil {
		return errorJSON(w, http.StatusBadRequest, "registration session expired")
	}

	user, err := a.repo.GetUserByID(ctx, userID)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to get user", err)
	}

	credential, err := a.webauthn.WebAuthn().FinishRegistration(user, *sessionData, r)
	if err != nil {
		slog.Error("registration failed", "error", err)
		return errorJSON(w, http.StatusBadRequest, "registration failed")
	}

	dbCred := NewCredentialFromWebAuthn(user.ID, credential)
	if createErr := a.repo.CreateCredential(ctx, dbCred); createErr != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to store credential", createErr)
	}

	codes, err := a.generateAndStoreRecoveryCodes(ctx, user.ID)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to generate recovery codes", err)
	}

	// Email mode: send verification email and redirect to pending page.
	if a.UseEmailMode() && user.Email != nil && a.config.RequireVerification {
		plainToken, tokenHash, expiresAt, tokenErr := GenerateToken()
		if tokenErr != nil {
			return errorJSONLog(w, http.StatusInternalServerError, "failed to generate verification token", tokenErr)
		}
		if tokenErr = a.repo.CreateEmailVerificationToken(ctx, user.ID, tokenHash, expiresAt); tokenErr != nil {
			return errorJSONLog(w, http.StatusInternalServerError, "failed to store verification token", tokenErr)
		}

		verifyURL := a.config.BaseURL + "/auth/verify-email?token=" + plainToken
		if err := a.enqueueEmail(r.Context(), "verification", *user.Email, verifyURL); err != nil {
			slog.Error("failed to enqueue verification email", "error", err, "email", *user.Email)
		}

		redirectAfterAck := a.redirectTarget(r)

		if err := session.Set(w, r, "recovery_codes", codes); err != nil {
			return errorJSONLog(w, http.StatusInternalServerError, "failed to store recovery codes", err)
		}
		if err := session.Set(w, r, "redirect_after_login", redirectAfterAck); err != nil {
			return errorJSONLog(w, http.StatusInternalServerError, "failed to store redirect", err)
		}

		return burrow.JSON(w, http.StatusOK, map[string]any{
			"status":   "ok",
			"redirect": "/auth/recovery-codes",
		})
	}

	// Username mode: create session immediately.
	redirectAfterAck := a.redirectTarget(r)
	if err := session.Save(w, r, map[string]any{
		"user_id":              user.ID,
		"recovery_codes":       codes,
		"redirect_after_login": redirectAfterAck,
	}); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to create session", err)
	}

	return burrow.JSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"redirect": "/auth/recovery-codes",
	})
}

// --- Login ---

// LoginPage renders the login page.
func (a *App) LoginPage(w http.ResponseWriter, r *http.Request) error {
	return a.renderer.LoginPage(w, r, a.config.LoginRedirect)
}

// LoginBegin starts the WebAuthn discoverable login process.
func (a *App) LoginBegin(w http.ResponseWriter, r *http.Request) error {
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
func (a *App) LoginFinish(w http.ResponseWriter, r *http.Request) error {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		return errorJSON(w, http.StatusBadRequest, "session_id is required")
	}

	sessionData, err := a.webauthn.GetDiscoverableSession(sessionID)
	if err != nil {
		return errorJSON(w, http.StatusBadRequest, "login session expired")
	}

	var foundUser *User
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

// --- Logout ---

// Logout clears the session cookie.
func (a *App) Logout(w http.ResponseWriter, r *http.Request) error {
	session.Clear(w, r)
	htmx.SmartRedirect(w, r, a.config.LogoutRedirect)
	return nil
}

// --- Credentials ---

// CredentialsPage renders the credentials management page.
func (a *App) CredentialsPage(w http.ResponseWriter, r *http.Request) error {
	user := MustCurrentUser(r.Context())
	creds, err := a.repo.GetCredentialsByUserID(r.Context(), user.ID)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to get credentials", err)
	}
	return a.renderer.CredentialsPage(w, r, creds)
}

// AddCredentialBegin starts the process of adding a new credential.
func (a *App) AddCredentialBegin(w http.ResponseWriter, r *http.Request) error {
	user := MustCurrentUser(r.Context())
	options, sessionData, err := a.webauthn.WebAuthn().BeginRegistration(user)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to begin registration", err)
	}
	a.webauthn.StoreRegistrationSession(user.ID, sessionData)

	return burrow.JSON(w, http.StatusOK, map[string]any{"publicKey": options.Response})
}

// AddCredentialFinish completes adding a new credential.
func (a *App) AddCredentialFinish(w http.ResponseWriter, r *http.Request) error {
	user := MustCurrentUser(r.Context())
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
func (a *App) DeleteCredential(w http.ResponseWriter, r *http.Request) error {
	user := MustCurrentUser(r.Context())
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

// --- Recovery ---

// RecoveryLoginRequest is the request body for recovery login.
type RecoveryLoginRequest struct {
	Username string `json:"username" form:"username"`
	Code     string `json:"code" form:"code"`
}

// RecoveryPage renders the recovery login page.
func (a *App) RecoveryPage(w http.ResponseWriter, r *http.Request) error {
	return a.renderer.RecoveryPage(w, r, a.config.LoginRedirect)
}

// RecoveryLogin authenticates a user with a recovery code.
func (a *App) RecoveryLogin(w http.ResponseWriter, r *http.Request) error {
	var req RecoveryLoginRequest
	if err := burrow.Bind(r, &req); err != nil {
		return errorJSON(w, http.StatusBadRequest, "invalid request")
	}

	if req.Username == "" || req.Code == "" {
		return errorJSON(w, http.StatusBadRequest, "username and code are required")
	}

	user, err := a.repo.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		// Run a dummy bcrypt comparison to prevent timing side-channel
		// that would reveal whether the username exists.
		_ = bcrypt.CompareHashAndPassword(
			[]byte("$2a$12$000000000000000000000000000000000000000000000000000000"),
			[]byte(req.Code),
		)
		return errorJSON(w, http.StatusUnauthorized, "invalid username or recovery code")
	}

	if !user.IsActive {
		return errorJSON(w, http.StatusForbidden, "account deactivated")
	}

	normalizedCode := NormalizeCode(req.Code)
	valid, err := a.repo.ValidateAndUseRecoveryCode(r.Context(), user.ID, normalizedCode)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "validation error", err)
	}
	if !valid {
		return errorJSON(w, http.StatusUnauthorized, "invalid username or recovery code")
	}

	// Read redirect target BEFORE session.Save() which replaces all session values.
	redirect := a.redirectTarget(r)

	if err := session.Save(w, r, map[string]any{"user_id": user.ID}); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to create session", err)
	}

	remaining, _ := a.repo.GetUnusedRecoveryCodeCount(r.Context(), user.ID)

	return burrow.JSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"remaining_codes": remaining,
		"redirect":        redirect,
	})
}

// RegenerateRecoveryCodes generates new recovery codes and invalidates old ones.
// Stores codes in session and returns a redirect to the recovery codes page.
func (a *App) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) error {
	user := MustCurrentUser(r.Context())
	codes, err := a.generateAndStoreRecoveryCodes(r.Context(), user.ID)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to regenerate codes", err)
	}

	if err := session.Set(w, r, "recovery_codes", codes); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to store recovery codes", err)
	}
	if err := session.Set(w, r, "redirect_after_login", "/auth/credentials"); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to store redirect", err)
	}

	return burrow.JSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"redirect": "/auth/recovery-codes",
	})
}

// RecoveryCodesPage renders the dedicated recovery codes page.
// Codes are read from the session; if none are present, redirects to login redirect.
func (a *App) RecoveryCodesPage(w http.ResponseWriter, r *http.Request) error {
	values := session.GetValues(r)
	codesRaw, ok := values["recovery_codes"]
	if !ok {
		htmx.SmartRedirect(w, r, a.config.LoginRedirect)
		return nil
	}

	codes, ok := codesRaw.([]string)
	if !ok || len(codes) == 0 {
		http.Redirect(w, r, a.config.LoginRedirect, http.StatusSeeOther)
		return nil
	}

	return a.renderer.RecoveryCodesPage(w, r, codes)
}

// AcknowledgeRecoveryCodes clears recovery codes from the session and redirects.
func (a *App) AcknowledgeRecoveryCodes(w http.ResponseWriter, r *http.Request) error {
	redirect := a.redirectTarget(r)

	if err := session.Delete(w, r, "recovery_codes"); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to clear recovery codes", err)
	}
	if err := session.Delete(w, r, "redirect_after_login"); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to clear redirect", err)
	}

	htmx.SmartRedirect(w, r, redirect)
	return nil
}

// --- Email verification ---

// VerifyPendingPage renders the "check your email" page.
func (a *App) VerifyPendingPage(w http.ResponseWriter, r *http.Request) error {
	return a.renderer.VerifyPendingPage(w, r)
}

// VerifyEmail handles the email verification link.
func (a *App) VerifyEmail(w http.ResponseWriter, r *http.Request) error {
	token := r.URL.Query().Get("token")
	if token == "" {
		return a.renderer.VerifyEmailErrorPage(w, r, "missing_token")
	}

	ctx := r.Context()
	tokenHash := HashToken(token)

	verificationToken, err := a.repo.GetEmailVerificationToken(ctx, tokenHash)
	if err != nil {
		return a.renderer.VerifyEmailErrorPage(w, r, "invalid_token")
	}

	if time.Now().After(verificationToken.ExpiresAt) {
		if delErr := a.repo.DeleteEmailVerificationToken(ctx, verificationToken.ID); delErr != nil {
			slog.Error("failed to delete expired verification token", "token_id", verificationToken.ID, "error", delErr)
		}
		return a.renderer.VerifyEmailErrorPage(w, r, "token_expired")
	}

	if markErr := a.repo.MarkEmailVerified(ctx, verificationToken.UserID); markErr != nil {
		slog.Error("failed to mark email as verified", "error", markErr)
		return a.renderer.VerifyEmailErrorPage(w, r, "verification_failed")
	}

	if delErr := a.repo.DeleteUserEmailVerificationTokens(ctx, verificationToken.UserID); delErr != nil {
		slog.Error("failed to delete verification tokens after verify", "user_id", verificationToken.UserID, "error", delErr)
	}

	user, err := a.repo.GetUserByID(ctx, verificationToken.UserID)
	if err != nil {
		slog.Error("failed to get user after verification", "error", err)
		return a.renderer.VerifyEmailErrorPage(w, r, "verification_failed")
	}

	if err := session.Save(w, r, map[string]any{"user_id": user.ID}); err != nil {
		slog.Error("failed to create session after verification", "error", err)
		return a.renderer.VerifyEmailErrorPage(w, r, "verification_failed")
	}

	return a.renderer.VerifyEmailSuccessPage(w, r)
}

// ResendVerificationRequest is the request body for resending verification email.
type ResendVerificationRequest struct {
	Email string `json:"email" form:"email"`
}

// ResendVerification resends the verification email.
func (a *App) ResendVerification(w http.ResponseWriter, r *http.Request) error {
	var req ResendVerificationRequest
	if err := burrow.Bind(r, &req); err != nil {
		return errorJSON(w, http.StatusBadRequest, "invalid request")
	}
	if req.Email == "" {
		return errorJSON(w, http.StatusBadRequest, "email is required")
	}

	ctx := r.Context()

	user, err := a.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return burrow.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
	if user.EmailVerified {
		return burrow.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}

	if user.Email == nil {
		return burrow.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}

	if delErr := a.repo.DeleteUserEmailVerificationTokens(ctx, user.ID); delErr != nil {
		slog.Error("failed to delete old verification tokens before resend", "user_id", user.ID, "error", delErr)
	}

	plainToken, tokenHash, expiresAt, tokenErr := GenerateToken()
	if tokenErr != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to send verification email", tokenErr)
	}
	if tokenErr = a.repo.CreateEmailVerificationToken(ctx, user.ID, tokenHash, expiresAt); tokenErr != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to send verification email", tokenErr)
	}

	verifyURL := a.config.BaseURL + "/auth/verify-email?token=" + plainToken
	if err := a.enqueueEmail(r.Context(), "verification", *user.Email, verifyURL); err != nil {
		slog.Error("failed to enqueue verification email", "error", err, "email", *user.Email)
	}

	return burrow.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// redirectTarget reads "redirect_after_login" from the session and validates it,
// falling back to the configured login redirect.
func (a *App) redirectTarget(r *http.Request) string {
	return SafeRedirectPath(session.GetString(r, "redirect_after_login"), a.config.LoginRedirect)
}

// --- Internal helpers ---

func (a *App) generateAndStoreRecoveryCodes(ctx context.Context, userID string) ([]string, error) {
	if err := a.repo.DeleteRecoveryCodes(ctx, userID); err != nil {
		return nil, err
	}

	codes, hashes, err := a.recovery.GenerateCodes(CodeCount)
	if err != nil {
		return nil, err
	}

	if err := a.repo.CreateRecoveryCodes(ctx, userID, hashes); err != nil {
		return nil, err
	}

	return codes, nil
}

func (a *App) validateInviteToken(ctx context.Context, token string) (*Invite, error) {
	tokenHash := HashToken(token)
	return a.repo.GetInviteByTokenHash(ctx, tokenHash)
}

func (a *App) isFirstUser(ctx context.Context) (bool, error) {
	count, err := a.repo.CountUsers(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
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

// errSignCountRegressed indicates a possible cloned credential.
var errSignCountRegressed = errors.New("sign count did not increase")

// verifySignCount checks whether the incoming sign count from the authenticator
// is consistent with the stored value. Software authenticators (e.g. 1Password,
// iCloud Keychain) always report 0, so both-zero is accepted. A non-increasing
// count when the stored value is nonzero indicates a possible cloned credential.
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
