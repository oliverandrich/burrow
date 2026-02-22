package auth

import (
	"context"
	"encoding/binary"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/session"
	"github.com/go-chi/chi/v5"
	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
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
	VerifyEmailSuccess(w http.ResponseWriter, r *http.Request) error
	VerifyEmailError(w http.ResponseWriter, r *http.Request, errorCode string) error
}

// Handlers contains all auth and invite HTTP handlers.
type Handlers struct {
	repo     *Repository
	webauthn WebAuthnService
	recovery *RecoveryService
	email    EmailService // nil if email mode is disabled
	renderer Renderer
	config   *Config
}

// NewHandlers creates a new Handlers instance.
// email can be nil if email mode is disabled.
func NewHandlers(
	repo *Repository,
	wa WebAuthnService,
	email EmailService,
	renderer Renderer,
	config *Config,
) *Handlers {
	return &Handlers{
		repo:     repo,
		webauthn: wa,
		recovery: NewRecoveryService(),
		email:    email,
		renderer: renderer,
		config:   config,
	}
}

// UseEmailMode returns true if email-based authentication is enabled.
func (h *Handlers) UseEmailMode() bool {
	return h.config != nil && h.config.UseEmail
}

// IsInviteOnly returns true if invite-only registration is enabled.
func (h *Handlers) IsInviteOnly() bool {
	return h.config != nil && h.config.InviteOnly
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
func (h *Handlers) RegisterPage(w http.ResponseWriter, r *http.Request) error {
	inviteToken := r.URL.Query().Get("invite")

	if h.IsInviteOnly() && inviteToken != "" {
		invite, err := h.validateInviteToken(r.Context(), inviteToken)
		if err != nil || !invite.IsValid() {
			return h.renderer.RegisterPage(w, r, h.UseEmailMode(), true, "", "")
		}
		return h.renderer.RegisterPage(w, r, h.UseEmailMode(), true, invite.Email, inviteToken)
	}

	return h.renderer.RegisterPage(w, r, h.UseEmailMode(), h.IsInviteOnly(), "", "")
}

// RegisterBegin starts the WebAuthn registration process.
func (h *Handlers) RegisterBegin(w http.ResponseWriter, r *http.Request) error {
	var req RegisterBeginRequest
	if err := burrow.Bind(r, &req); err != nil {
		return errorJSON(w, http.StatusBadRequest, "invalid request")
	}

	ctx := r.Context()

	// Invite-only mode: validate invite token (first user bypasses).
	var validInvite *Invite
	if h.IsInviteOnly() {
		isFirst, err := h.isFirstUser(ctx)
		if err != nil {
			return errorJSONLog(w, http.StatusInternalServerError, "database error", err)
		}
		if !isFirst {
			if req.Invite == "" {
				return errorJSON(w, http.StatusForbidden, "invite token required")
			}
			invite, validateErr := h.validateInviteToken(ctx, req.Invite)
			if validateErr != nil || !invite.IsValid() {
				return errorJSON(w, http.StatusForbidden, "invalid or expired invite")
			}
			validInvite = invite

			if h.UseEmailMode() && req.Email != invite.Email {
				return errorJSON(w, http.StatusForbidden, "email does not match invite")
			}
		}
	}

	var user *User
	var createErr error

	if h.UseEmailMode() {
		if req.Email == "" {
			return errorJSON(w, http.StatusBadRequest, "email is required")
		}
		exists, err := h.repo.EmailExists(ctx, req.Email)
		if err != nil {
			return errorJSONLog(w, http.StatusInternalServerError, "database error", err)
		}
		if exists {
			return errorJSON(w, http.StatusConflict, "registration failed")
		}
		user, createErr = h.repo.CreateUserWithEmail(ctx, req.Email, req.Name)
	} else {
		if req.Username == "" {
			return errorJSON(w, http.StatusBadRequest, "username is required")
		}
		exists, err := h.repo.UserExists(ctx, req.Username)
		if err != nil {
			return errorJSONLog(w, http.StatusInternalServerError, "database error", err)
		}
		if exists {
			return errorJSON(w, http.StatusConflict, "registration failed")
		}
		user, createErr = h.repo.CreateUser(ctx, req.Username, req.Name)
	}

	if createErr != nil {
		slog.Error("failed to create user", "error", createErr)
		return errorJSON(w, http.StatusInternalServerError, "failed to create user")
	}

	// Promote the first registered user to admin.
	count, countErr := h.repo.CountUsers(ctx)
	if countErr == nil && count == 1 {
		_ = h.repo.SetUserRole(ctx, user.ID, RoleAdmin)
		user.Role = RoleAdmin
		slog.Info("first user registered as admin", "user_id", user.ID) //nolint:gosec // G706: user_id is safe
	}

	// Mark invite as used.
	if validInvite != nil {
		if markErr := h.repo.MarkInviteUsed(ctx, validInvite.ID, user.ID); markErr != nil {
			slog.Error("failed to mark invite as used", "invite_id", validInvite.ID) //nolint:gosec // G706: invite_id is int
		}
	}

	// Begin WebAuthn registration.
	options, sessionData, err := h.webauthn.WebAuthn().BeginRegistration(user)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to begin registration", err)
	}
	h.webauthn.StoreRegistrationSession(user.ID, sessionData)

	return burrow.JSON(w, http.StatusOK, map[string]any{
		"publicKey": options.Response,
		"user_id":   user.ID,
	})
}

// RegisterFinish completes the WebAuthn registration process.
func (h *Handlers) RegisterFinish(w http.ResponseWriter, r *http.Request) error {
	userID, err := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	if err != nil {
		return errorJSON(w, http.StatusBadRequest, "invalid user_id")
	}

	ctx := r.Context()

	sessionData, err := h.webauthn.GetRegistrationSession(userID)
	if err != nil {
		return errorJSON(w, http.StatusBadRequest, "registration session expired")
	}

	user, err := h.repo.GetUserByID(ctx, userID)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to get user", err)
	}

	credential, err := h.webauthn.WebAuthn().FinishRegistration(user, *sessionData, r)
	if err != nil {
		slog.Error("registration failed", "error", err)
		return errorJSON(w, http.StatusBadRequest, "registration failed")
	}

	dbCred := NewCredentialFromWebAuthn(user.ID, credential)
	if createErr := h.repo.CreateCredential(ctx, dbCred); createErr != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to store credential", createErr)
	}

	codes, err := h.generateAndStoreRecoveryCodes(ctx, user.ID)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to generate recovery codes", err)
	}

	// Email mode: send verification email and redirect to pending page.
	if h.UseEmailMode() && user.Email != nil && h.config.RequireVerification {
		plainToken, tokenHash, expiresAt, tokenErr := h.email.GenerateToken()
		if tokenErr != nil {
			return errorJSONLog(w, http.StatusInternalServerError, "failed to generate verification token", tokenErr)
		}
		if tokenErr = h.repo.CreateEmailVerificationToken(ctx, user.ID, tokenHash, expiresAt); tokenErr != nil {
			return errorJSONLog(w, http.StatusInternalServerError, "failed to store verification token", tokenErr)
		}

		go func() {
			sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if sendErr := h.email.SendVerification(sendCtx, *user.Email, plainToken); sendErr != nil {
				slog.Error("failed to send verification email", "error", sendErr, "email", *user.Email)
			}
		}()

		redirectAfterAck := h.redirectTarget(r)

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
	redirectAfterAck := h.redirectTarget(r)
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
func (h *Handlers) LoginPage(w http.ResponseWriter, r *http.Request) error {
	return h.renderer.LoginPage(w, r, h.config.LoginRedirect)
}

// LoginBegin starts the WebAuthn discoverable login process.
func (h *Handlers) LoginBegin(w http.ResponseWriter, r *http.Request) error {
	options, sessionData, err := h.webauthn.WebAuthn().BeginDiscoverableLogin()
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to begin login", err)
	}

	sessionID := uuid.New().String()
	h.webauthn.StoreDiscoverableSession(sessionID, sessionData)

	return burrow.JSON(w, http.StatusOK, map[string]any{
		"publicKey":  options.Response,
		"session_id": sessionID,
	})
}

// LoginFinish completes the WebAuthn discoverable login.
func (h *Handlers) LoginFinish(w http.ResponseWriter, r *http.Request) error {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		return errorJSON(w, http.StatusBadRequest, "session_id is required")
	}

	sessionData, err := h.webauthn.GetDiscoverableSession(sessionID)
	if err != nil {
		return errorJSON(w, http.StatusBadRequest, "login session expired")
	}

	var foundUser *User
	credential, finishErr := h.webauthn.WebAuthn().FinishDiscoverableLogin(
		func(rawID, userHandle []byte) (gowebauthn.User, error) {
			if len(userHandle) < 8 {
				return nil, burrow.NewHTTPError(http.StatusBadRequest, "invalid user handle")
			}
			userID := int64(binary.BigEndian.Uint64(userHandle)) //nolint:gosec // user IDs are always positive
			user, userErr := h.repo.GetUserByIDWithCredentials(r.Context(), userID)
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

	if updateErr := h.repo.UpdateCredentialSignCount(r.Context(), credential.ID, credential.Authenticator.SignCount); updateErr != nil {
		slog.Warn("failed to update credential sign count", "error", updateErr)
	}

	if h.UseEmailMode() && h.config.RequireVerification && !foundUser.EmailVerified {
		return burrow.JSON(w, http.StatusForbidden, map[string]any{
			"error":    "email_not_verified",
			"redirect": "/auth/verify-pending",
		})
	}

	// Read redirect target BEFORE session.Save() which replaces all session values.
	redirect := h.redirectTarget(r)

	if err := session.Save(w, r, map[string]any{"user_id": foundUser.ID}); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to create session", err)
	}

	return burrow.JSON(w, http.StatusOK, map[string]string{"status": "ok", "redirect": redirect})
}

// --- Logout ---

// Logout clears the session cookie.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) error {
	session.Clear(w, r)
	http.Redirect(w, r, h.config.LogoutRedirect, http.StatusSeeOther)
	return nil
}

// --- Credentials ---

// CredentialsPage renders the credentials management page.
func (h *Handlers) CredentialsPage(w http.ResponseWriter, r *http.Request) error {
	user := GetUser(r)
	creds, err := h.repo.GetCredentialsByUserID(r.Context(), user.ID)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to get credentials", err)
	}
	return h.renderer.CredentialsPage(w, r, creds)
}

// AddCredentialBegin starts the process of adding a new credential.
func (h *Handlers) AddCredentialBegin(w http.ResponseWriter, r *http.Request) error {
	user := GetUser(r)
	options, sessionData, err := h.webauthn.WebAuthn().BeginRegistration(user)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to begin registration", err)
	}
	h.webauthn.StoreRegistrationSession(user.ID, sessionData)

	return burrow.JSON(w, http.StatusOK, map[string]any{"publicKey": options.Response})
}

// AddCredentialFinish completes adding a new credential.
func (h *Handlers) AddCredentialFinish(w http.ResponseWriter, r *http.Request) error {
	user := GetUser(r)
	sessionData, err := h.webauthn.GetRegistrationSession(user.ID)
	if err != nil {
		return errorJSON(w, http.StatusBadRequest, "registration session expired")
	}

	credential, err := h.webauthn.WebAuthn().FinishRegistration(user, *sessionData, r)
	if err != nil {
		slog.Error("registration failed", "error", err)
		return errorJSON(w, http.StatusBadRequest, "registration failed")
	}

	dbCred := NewCredentialFromWebAuthn(user.ID, credential)
	if err := h.repo.CreateCredential(r.Context(), dbCred); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to store credential", err)
	}

	return burrow.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteCredential removes a credential.
func (h *Handlers) DeleteCredential(w http.ResponseWriter, r *http.Request) error {
	user := GetUser(r)
	credID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errorJSON(w, http.StatusBadRequest, "invalid credential id")
	}

	count, err := h.repo.CountUserCredentials(r.Context(), user.ID)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "database error", err)
	}
	if count <= 1 {
		return errorJSON(w, http.StatusBadRequest, "cannot delete last credential")
	}

	if err := h.repo.DeleteCredential(r.Context(), credID, user.ID); err != nil {
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
func (h *Handlers) RecoveryPage(w http.ResponseWriter, r *http.Request) error {
	return h.renderer.RecoveryPage(w, r, h.config.LoginRedirect)
}

// RecoveryLogin authenticates a user with a recovery code.
func (h *Handlers) RecoveryLogin(w http.ResponseWriter, r *http.Request) error {
	var req RecoveryLoginRequest
	if err := burrow.Bind(r, &req); err != nil {
		return errorJSON(w, http.StatusBadRequest, "invalid request")
	}

	if req.Username == "" || req.Code == "" {
		return errorJSON(w, http.StatusBadRequest, "username and code are required")
	}

	user, err := h.repo.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		return errorJSON(w, http.StatusUnauthorized, "invalid username or recovery code")
	}

	normalizedCode := NormalizeCode(req.Code)
	valid, err := h.repo.ValidateAndUseRecoveryCode(r.Context(), user.ID, normalizedCode)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "validation error", err)
	}
	if !valid {
		return errorJSON(w, http.StatusUnauthorized, "invalid username or recovery code")
	}

	// Read redirect target BEFORE session.Save() which replaces all session values.
	redirect := h.redirectTarget(r)

	if err := session.Save(w, r, map[string]any{"user_id": user.ID}); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to create session", err)
	}

	remaining, _ := h.repo.GetUnusedRecoveryCodeCount(r.Context(), user.ID)

	return burrow.JSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"remaining_codes": remaining,
		"redirect":        redirect,
	})
}

// RegenerateRecoveryCodes generates new recovery codes and invalidates old ones.
// Stores codes in session and returns a redirect to the recovery codes page.
func (h *Handlers) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) error {
	user := GetUser(r)
	codes, err := h.generateAndStoreRecoveryCodes(r.Context(), user.ID)
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
func (h *Handlers) RecoveryCodesPage(w http.ResponseWriter, r *http.Request) error {
	values := session.GetValues(r)
	codesRaw, ok := values["recovery_codes"]
	if !ok {
		http.Redirect(w, r, h.config.LoginRedirect, http.StatusSeeOther)
		return nil
	}

	codes, ok := codesRaw.([]string)
	if !ok || len(codes) == 0 {
		http.Redirect(w, r, h.config.LoginRedirect, http.StatusSeeOther)
		return nil
	}

	return h.renderer.RecoveryCodesPage(w, r, codes)
}

// AcknowledgeRecoveryCodes clears recovery codes from the session and redirects.
func (h *Handlers) AcknowledgeRecoveryCodes(w http.ResponseWriter, r *http.Request) error {
	redirect := h.redirectTarget(r)

	if err := session.Delete(w, r, "recovery_codes"); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to clear recovery codes", err)
	}
	if err := session.Delete(w, r, "redirect_after_login"); err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to clear redirect", err)
	}

	http.Redirect(w, r, redirect, http.StatusSeeOther)
	return nil
}

// --- Email verification ---

// VerifyPendingPage renders the "check your email" page.
func (h *Handlers) VerifyPendingPage(w http.ResponseWriter, r *http.Request) error {
	return h.renderer.VerifyPendingPage(w, r)
}

// VerifyEmail handles the email verification link.
func (h *Handlers) VerifyEmail(w http.ResponseWriter, r *http.Request) error {
	token := r.URL.Query().Get("token")
	if token == "" {
		return h.renderer.VerifyEmailError(w, r, "missing_token")
	}

	ctx := r.Context()
	tokenHash := HashToken(token)

	verificationToken, err := h.repo.GetEmailVerificationToken(ctx, tokenHash)
	if err != nil {
		return h.renderer.VerifyEmailError(w, r, "invalid_token")
	}

	if time.Now().After(verificationToken.ExpiresAt) {
		_ = h.repo.DeleteEmailVerificationToken(ctx, verificationToken.ID)
		return h.renderer.VerifyEmailError(w, r, "token_expired")
	}

	if markErr := h.repo.MarkEmailVerified(ctx, verificationToken.UserID); markErr != nil {
		slog.Error("failed to mark email as verified", "error", markErr)
		return h.renderer.VerifyEmailError(w, r, "verification_failed")
	}

	_ = h.repo.DeleteUserEmailVerificationTokens(ctx, verificationToken.UserID)

	user, err := h.repo.GetUserByID(ctx, verificationToken.UserID)
	if err != nil {
		slog.Error("failed to get user after verification", "error", err)
		return h.renderer.VerifyEmailError(w, r, "verification_failed")
	}

	if err := session.Save(w, r, map[string]any{"user_id": user.ID}); err != nil {
		slog.Error("failed to create session after verification", "error", err)
		return h.renderer.VerifyEmailError(w, r, "verification_failed")
	}

	return h.renderer.VerifyEmailSuccess(w, r)
}

// ResendVerificationRequest is the request body for resending verification email.
type ResendVerificationRequest struct {
	Email string `json:"email" form:"email"`
}

// ResendVerification resends the verification email.
func (h *Handlers) ResendVerification(w http.ResponseWriter, r *http.Request) error {
	var req ResendVerificationRequest
	if err := burrow.Bind(r, &req); err != nil {
		return errorJSON(w, http.StatusBadRequest, "invalid request")
	}
	if req.Email == "" {
		return errorJSON(w, http.StatusBadRequest, "email is required")
	}

	ctx := r.Context()

	user, err := h.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return burrow.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
	if user.EmailVerified {
		return burrow.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}

	_ = h.repo.DeleteUserEmailVerificationTokens(ctx, user.ID)

	plainToken, tokenHash, expiresAt, tokenErr := h.email.GenerateToken()
	if tokenErr != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to send verification email", tokenErr)
	}
	if tokenErr = h.repo.CreateEmailVerificationToken(ctx, user.ID, tokenHash, expiresAt); tokenErr != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to send verification email", tokenErr)
	}

	go func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if sendErr := h.email.SendVerification(sendCtx, *user.Email, plainToken); sendErr != nil {
			slog.Error("failed to send verification email", "error", sendErr, "email", *user.Email)
		}
	}()

	return burrow.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// redirectTarget reads "redirect_after_login" from the session and validates it,
// falling back to the configured login redirect.
func (h *Handlers) redirectTarget(r *http.Request) string {
	return SafeRedirectPath(session.GetString(r, "redirect_after_login"), h.config.LoginRedirect)
}

// --- Internal helpers ---

func (h *Handlers) generateAndStoreRecoveryCodes(ctx context.Context, userID int64) ([]string, error) {
	if err := h.repo.DeleteRecoveryCodes(ctx, userID); err != nil {
		return nil, err
	}

	codes, hashes, err := h.recovery.GenerateCodes(CodeCount)
	if err != nil {
		return nil, err
	}

	if err := h.repo.CreateRecoveryCodes(ctx, userID, hashes); err != nil {
		return nil, err
	}

	return codes, nil
}

func (h *Handlers) validateInviteToken(ctx context.Context, token string) (*Invite, error) {
	tokenHash := HashToken(token)
	return h.repo.GetInviteByTokenHash(ctx, tokenHash)
}

func (h *Handlers) isFirstUser(ctx context.Context) (bool, error) {
	count, err := h.repo.CountUsers(ctx)
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
