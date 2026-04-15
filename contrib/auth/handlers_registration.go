package auth

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/session"
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

	// Promote to admin if no admin exists yet.
	adminCount, countErr := a.repo.CountAdminUsers(ctx)
	if countErr == nil && adminCount == 0 {
		if roleErr := a.repo.SetUserRole(ctx, user.ID, RoleAdmin); roleErr != nil {
			slog.Error("failed to promote first user to admin", "user_id", user.ID, "error", roleErr)
		}
		user.Role = RoleAdmin
		slog.Info("first user registered as admin", "user_id", user.ID)
	}

	// Mark invite as used.
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
