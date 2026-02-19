package auth

import (
	"context"
	"encoding/binary"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/oliverandrich/go-webapp-template/contrib/session"
	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

// Renderer defines the page rendering interface for auth templates.
// Projects implement this to provide their own template rendering.
type Renderer interface {
	RegisterPage(c *echo.Context, useEmail, inviteOnly bool, email, invite string) error
	LoginPage(c *echo.Context, loginRedirect string) error
	CredentialsPage(c *echo.Context, creds []Credential) error
	RecoveryPage(c *echo.Context, loginRedirect string) error
	VerifyPendingPage(c *echo.Context) error
	VerifyEmailSuccess(c *echo.Context) error
	VerifyEmailError(c *echo.Context, errorCode string) error
	InvitesPage(c *echo.Context, invites []Invite, createdURL string, useEmail bool) error
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
func (h *Handlers) RegisterPage(c *echo.Context) error {
	inviteToken := c.QueryParam("invite")

	if h.IsInviteOnly() && inviteToken != "" {
		invite, err := h.validateInviteToken(c.Request().Context(), inviteToken)
		if err != nil || !invite.IsValid() {
			return h.renderer.RegisterPage(c, h.UseEmailMode(), true, "", "")
		}
		return h.renderer.RegisterPage(c, h.UseEmailMode(), true, invite.Email, inviteToken)
	}

	return h.renderer.RegisterPage(c, h.UseEmailMode(), h.IsInviteOnly(), "", "")
}

// RegisterBegin starts the WebAuthn registration process.
func (h *Handlers) RegisterBegin(c *echo.Context) error {
	var req RegisterBeginRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request")
	}

	ctx := c.Request().Context()

	// Invite-only mode: validate invite token (first user bypasses).
	var validInvite *Invite
	if h.IsInviteOnly() {
		isFirst, err := h.isFirstUser(ctx)
		if err != nil {
			return errorJSONLog(c, http.StatusInternalServerError, "database error", err)
		}
		if !isFirst {
			if req.Invite == "" {
				return errorJSON(c, http.StatusForbidden, "invite token required")
			}
			invite, validateErr := h.validateInviteToken(ctx, req.Invite)
			if validateErr != nil || !invite.IsValid() {
				return errorJSON(c, http.StatusForbidden, "invalid or expired invite")
			}
			validInvite = invite

			if h.UseEmailMode() && req.Email != invite.Email {
				return errorJSON(c, http.StatusForbidden, "email does not match invite")
			}
		}
	}

	var user *User
	var createErr error

	if h.UseEmailMode() {
		if req.Email == "" {
			return errorJSON(c, http.StatusBadRequest, "email is required")
		}
		exists, err := h.repo.EmailExists(ctx, req.Email)
		if err != nil {
			return errorJSONLog(c, http.StatusInternalServerError, "database error", err)
		}
		if exists {
			return errorJSON(c, http.StatusConflict, "registration failed")
		}
		user, createErr = h.repo.CreateUserWithEmail(ctx, req.Email, req.Name)
	} else {
		if req.Username == "" {
			return errorJSON(c, http.StatusBadRequest, "username is required")
		}
		exists, err := h.repo.UserExists(ctx, req.Username)
		if err != nil {
			return errorJSONLog(c, http.StatusInternalServerError, "database error", err)
		}
		if exists {
			return errorJSON(c, http.StatusConflict, "registration failed")
		}
		user, createErr = h.repo.CreateUser(ctx, req.Username, req.Name)
	}

	if createErr != nil {
		slog.Error("failed to create user", "error", createErr)
		return errorJSON(c, http.StatusInternalServerError, "failed to create user")
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
		return errorJSONLog(c, http.StatusInternalServerError, "failed to begin registration", err)
	}
	h.webauthn.StoreRegistrationSession(user.ID, sessionData)

	return c.JSON(http.StatusOK, map[string]any{
		"publicKey": options.Response,
		"user_id":   user.ID,
	})
}

// RegisterFinish completes the WebAuthn registration process.
func (h *Handlers) RegisterFinish(c *echo.Context) error {
	userID, err := strconv.ParseInt(c.QueryParam("user_id"), 10, 64)
	if err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid user_id")
	}

	ctx := c.Request().Context()

	sessionData, err := h.webauthn.GetRegistrationSession(userID)
	if err != nil {
		return errorJSON(c, http.StatusBadRequest, "registration session expired")
	}

	user, err := h.repo.GetUserByID(ctx, userID)
	if err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to get user", err)
	}

	credential, err := h.webauthn.WebAuthn().FinishRegistration(user, *sessionData, c.Request())
	if err != nil {
		slog.Error("registration failed", "error", err)
		return errorJSON(c, http.StatusBadRequest, "registration failed")
	}

	dbCred := NewCredentialFromWebAuthn(user.ID, credential)
	if createErr := h.repo.CreateCredential(ctx, dbCred); createErr != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to store credential", createErr)
	}

	codes, err := h.generateAndStoreRecoveryCodes(ctx, user.ID)
	if err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to generate recovery codes", err)
	}

	// Email mode: send verification email and redirect to pending page.
	if h.UseEmailMode() && user.Email != nil && h.config.RequireVerification {
		plainToken, tokenHash, expiresAt, tokenErr := h.email.GenerateToken()
		if tokenErr != nil {
			return errorJSONLog(c, http.StatusInternalServerError, "failed to generate verification token", tokenErr)
		}
		if tokenErr = h.repo.CreateEmailVerificationToken(ctx, user.ID, tokenHash, expiresAt); tokenErr != nil {
			return errorJSONLog(c, http.StatusInternalServerError, "failed to store verification token", tokenErr)
		}

		go func() {
			sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if sendErr := h.email.SendVerification(sendCtx, *user.Email, plainToken); sendErr != nil {
				slog.Error("failed to send verification email", "error", sendErr, "email", *user.Email)
			}
		}()

		return c.JSON(http.StatusOK, map[string]any{
			"status":         "ok",
			"redirect":       "/auth/verify-pending",
			"recovery_codes": codes,
		})
	}

	// Username mode: create session immediately.
	if err := session.Save(c, map[string]any{"user_id": user.ID}); err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to create session", err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"status":         "ok",
		"recovery_codes": codes,
	})
}

// --- Login ---

// LoginPage renders the login page.
func (h *Handlers) LoginPage(c *echo.Context) error {
	return h.renderer.LoginPage(c, h.config.LoginRedirect)
}

// LoginBegin starts the WebAuthn discoverable login process.
func (h *Handlers) LoginBegin(c *echo.Context) error {
	options, sessionData, err := h.webauthn.WebAuthn().BeginDiscoverableLogin()
	if err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to begin login", err)
	}

	sessionID := uuid.New().String()
	h.webauthn.StoreDiscoverableSession(sessionID, sessionData)

	return c.JSON(http.StatusOK, map[string]any{
		"publicKey":  options.Response,
		"session_id": sessionID,
	})
}

// LoginFinish completes the WebAuthn discoverable login.
func (h *Handlers) LoginFinish(c *echo.Context) error {
	sessionID := c.QueryParam("session_id")
	if sessionID == "" {
		return errorJSON(c, http.StatusBadRequest, "session_id is required")
	}

	sessionData, err := h.webauthn.GetDiscoverableSession(sessionID)
	if err != nil {
		return errorJSON(c, http.StatusBadRequest, "login session expired")
	}

	var foundUser *User
	credential, finishErr := h.webauthn.WebAuthn().FinishDiscoverableLogin(
		func(rawID, userHandle []byte) (gowebauthn.User, error) {
			if len(userHandle) < 8 {
				return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid user handle")
			}
			userID := int64(binary.BigEndian.Uint64(userHandle)) //nolint:gosec // user IDs are always positive
			user, userErr := h.repo.GetUserByIDWithCredentials(c.Request().Context(), userID)
			if userErr != nil {
				return nil, userErr
			}
			foundUser = user
			return user, nil
		},
		*sessionData,
		c.Request(),
	)
	if finishErr != nil {
		slog.Error("failed to finish discoverable login", "error", finishErr)
		return errorJSON(c, http.StatusUnauthorized, "login failed")
	}

	if updateErr := h.repo.UpdateCredentialSignCount(c.Request().Context(), credential.ID, credential.Authenticator.SignCount); updateErr != nil {
		slog.Warn("failed to update credential sign count", "error", updateErr)
	}

	if h.UseEmailMode() && h.config.RequireVerification && !foundUser.EmailVerified {
		return c.JSON(http.StatusForbidden, map[string]any{
			"error":    "email_not_verified",
			"redirect": "/auth/verify-pending",
		})
	}

	if err := session.Save(c, map[string]any{"user_id": foundUser.ID}); err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to create session", err)
	}

	next := c.QueryParam("next")
	redirect := SafeRedirectPath(next, h.config.LoginRedirect)

	return c.JSON(http.StatusOK, map[string]string{"status": "ok", "redirect": redirect})
}

// --- Logout ---

// Logout clears the session cookie.
func (h *Handlers) Logout(c *echo.Context) error {
	session.Clear(c)
	return c.Redirect(http.StatusSeeOther, "/")
}

// --- Credentials ---

// CredentialsPage renders the credentials management page.
func (h *Handlers) CredentialsPage(c *echo.Context) error {
	user := GetUser(c)
	creds, err := h.repo.GetCredentialsByUserID(c.Request().Context(), user.ID)
	if err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to get credentials", err)
	}
	return h.renderer.CredentialsPage(c, creds)
}

// AddCredentialBegin starts the process of adding a new credential.
func (h *Handlers) AddCredentialBegin(c *echo.Context) error {
	user := GetUser(c)
	options, sessionData, err := h.webauthn.WebAuthn().BeginRegistration(user)
	if err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to begin registration", err)
	}
	h.webauthn.StoreRegistrationSession(user.ID, sessionData)

	return c.JSON(http.StatusOK, map[string]any{"publicKey": options.Response})
}

// AddCredentialFinish completes adding a new credential.
func (h *Handlers) AddCredentialFinish(c *echo.Context) error {
	user := GetUser(c)
	sessionData, err := h.webauthn.GetRegistrationSession(user.ID)
	if err != nil {
		return errorJSON(c, http.StatusBadRequest, "registration session expired")
	}

	credential, err := h.webauthn.WebAuthn().FinishRegistration(user, *sessionData, c.Request())
	if err != nil {
		slog.Error("registration failed", "error", err)
		return errorJSON(c, http.StatusBadRequest, "registration failed")
	}

	dbCred := NewCredentialFromWebAuthn(user.ID, credential)
	if err := h.repo.CreateCredential(c.Request().Context(), dbCred); err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to store credential", err)
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteCredential removes a credential.
func (h *Handlers) DeleteCredential(c *echo.Context) error {
	user := GetUser(c)
	credID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid credential id")
	}

	count, err := h.repo.CountUserCredentials(c.Request().Context(), user.ID)
	if err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "database error", err)
	}
	if count <= 1 {
		return errorJSON(c, http.StatusBadRequest, "cannot delete last credential")
	}

	if err := h.repo.DeleteCredential(c.Request().Context(), credID, user.ID); err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to delete credential", err)
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// --- Recovery ---

// RecoveryLoginRequest is the request body for recovery login.
type RecoveryLoginRequest struct {
	Username string `json:"username" form:"username"`
	Code     string `json:"code" form:"code"`
}

// RecoveryPage renders the recovery login page.
func (h *Handlers) RecoveryPage(c *echo.Context) error {
	return h.renderer.RecoveryPage(c, h.config.LoginRedirect)
}

// RecoveryLogin authenticates a user with a recovery code.
func (h *Handlers) RecoveryLogin(c *echo.Context) error {
	var req RecoveryLoginRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request")
	}

	if req.Username == "" || req.Code == "" {
		return errorJSON(c, http.StatusBadRequest, "username and code are required")
	}

	user, err := h.repo.GetUserByUsername(c.Request().Context(), req.Username)
	if err != nil {
		return errorJSON(c, http.StatusUnauthorized, "invalid username or recovery code")
	}

	normalizedCode := NormalizeCode(req.Code)
	valid, err := h.repo.ValidateAndUseRecoveryCode(c.Request().Context(), user.ID, normalizedCode)
	if err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "validation error", err)
	}
	if !valid {
		return errorJSON(c, http.StatusUnauthorized, "invalid username or recovery code")
	}

	if err := session.Save(c, map[string]any{"user_id": user.ID}); err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to create session", err)
	}

	remaining, _ := h.repo.GetUnusedRecoveryCodeCount(c.Request().Context(), user.ID)

	next := c.QueryParam("next")
	redirect := SafeRedirectPath(next, h.config.LoginRedirect)

	return c.JSON(http.StatusOK, map[string]any{
		"status":          "ok",
		"remaining_codes": remaining,
		"redirect":        redirect,
	})
}

// RegenerateRecoveryCodes generates new recovery codes and invalidates old ones.
func (h *Handlers) RegenerateRecoveryCodes(c *echo.Context) error {
	user := GetUser(c)
	codes, err := h.generateAndStoreRecoveryCodes(c.Request().Context(), user.ID)
	if err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to regenerate codes", err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"status":         "ok",
		"recovery_codes": codes,
	})
}

// --- Email verification ---

// VerifyPendingPage renders the "check your email" page.
func (h *Handlers) VerifyPendingPage(c *echo.Context) error {
	return h.renderer.VerifyPendingPage(c)
}

// VerifyEmail handles the email verification link.
func (h *Handlers) VerifyEmail(c *echo.Context) error {
	token := c.QueryParam("token")
	if token == "" {
		return h.renderer.VerifyEmailError(c, "missing_token")
	}

	ctx := c.Request().Context()
	tokenHash := HashToken(token)

	verificationToken, err := h.repo.GetEmailVerificationToken(ctx, tokenHash)
	if err != nil {
		return h.renderer.VerifyEmailError(c, "invalid_token")
	}

	if time.Now().After(verificationToken.ExpiresAt) {
		_ = h.repo.DeleteEmailVerificationToken(ctx, verificationToken.ID)
		return h.renderer.VerifyEmailError(c, "token_expired")
	}

	if markErr := h.repo.MarkEmailVerified(ctx, verificationToken.UserID); markErr != nil {
		slog.Error("failed to mark email as verified", "error", markErr)
		return h.renderer.VerifyEmailError(c, "verification_failed")
	}

	_ = h.repo.DeleteUserEmailVerificationTokens(ctx, verificationToken.UserID)

	user, err := h.repo.GetUserByID(ctx, verificationToken.UserID)
	if err != nil {
		slog.Error("failed to get user after verification", "error", err)
		return h.renderer.VerifyEmailError(c, "verification_failed")
	}

	if err := session.Save(c, map[string]any{"user_id": user.ID}); err != nil {
		slog.Error("failed to create session after verification", "error", err)
		return h.renderer.VerifyEmailError(c, "verification_failed")
	}

	return h.renderer.VerifyEmailSuccess(c)
}

// ResendVerificationRequest is the request body for resending verification email.
type ResendVerificationRequest struct {
	Email string `json:"email" form:"email"`
}

// ResendVerification resends the verification email.
func (h *Handlers) ResendVerification(c *echo.Context) error {
	var req ResendVerificationRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request")
	}
	if req.Email == "" {
		return errorJSON(c, http.StatusBadRequest, "email is required")
	}

	ctx := c.Request().Context()

	user, err := h.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
	if user.EmailVerified {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}

	_ = h.repo.DeleteUserEmailVerificationTokens(ctx, user.ID)

	plainToken, tokenHash, expiresAt, tokenErr := h.email.GenerateToken()
	if tokenErr != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to send verification email", tokenErr)
	}
	if tokenErr = h.repo.CreateEmailVerificationToken(ctx, user.ID, tokenHash, expiresAt); tokenErr != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to send verification email", tokenErr)
	}

	go func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if sendErr := h.email.SendVerification(sendCtx, *user.Email, plainToken); sendErr != nil {
			slog.Error("failed to send verification email", "error", sendErr, "email", *user.Email)
		}
	}()

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// --- Invites ---

// CreateInviteRequest is the request body for creating an invite.
type CreateInviteRequest struct {
	Email string `form:"email"`
}

// InvitesPage renders the admin invites management page.
func (h *Handlers) InvitesPage(c *echo.Context) error {
	return h.renderInvitesPage(c, "")
}

// CreateInvite creates a new invite and optionally sends an email.
func (h *Handlers) CreateInvite(c *echo.Context) error {
	var req CreateInviteRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid request")
	}

	if h.UseEmailMode() && req.Email == "" {
		return errorJSON(c, http.StatusBadRequest, "email is required")
	}

	user := GetUser(c)
	if user == nil {
		return errorJSON(c, http.StatusUnauthorized, "unauthorized")
	}

	plainToken, tokenHash, err := GenerateInviteToken()
	if err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to generate invite token", err)
	}

	invite := &Invite{
		Email:     req.Email,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(InviteExpiry),
		CreatedBy: &user.ID,
	}
	if err := h.repo.CreateInvite(c.Request().Context(), invite); err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to create invite", err)
	}

	if h.email != nil && req.Email != "" {
		go func() {
			sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if sendErr := h.email.SendInvite(sendCtx, req.Email, plainToken); sendErr != nil {
				slog.Error("failed to send invite email", "error", sendErr, "email", req.Email)
			}
		}()
	}

	slog.Info("invite created", "email", req.Email, "created_by", user.Username)

	createdURL := h.config.BaseURL + "/auth/register?invite=" + plainToken
	return h.renderInvitesPage(c, createdURL)
}

// DeleteInvite revokes an invite by hard-deleting it.
func (h *Handlers) DeleteInvite(c *echo.Context) error {
	inviteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid invite id")
	}

	if err := h.repo.DeleteInvite(c.Request().Context(), inviteID); err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to delete invite", err)
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
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

func (h *Handlers) renderInvitesPage(c *echo.Context, createdURL string) error {
	invites, err := h.repo.ListInvites(c.Request().Context())
	if err != nil {
		return errorJSONLog(c, http.StatusInternalServerError, "failed to list invites", err)
	}
	return h.renderer.InvitesPage(c, invites, createdURL, h.UseEmailMode())
}

func errorJSON(c *echo.Context, statusCode int, msg string) error {
	return c.JSON(statusCode, map[string]string{"error": msg})
}

func errorJSONLog(c *echo.Context, statusCode int, msg string, err error) error { //nolint:unparam // statusCode is kept for consistency with errorJSON
	if err != nil {
		slog.Error(msg, "error", err)
	}
	return c.JSON(statusCode, map[string]string{"error": msg})
}
