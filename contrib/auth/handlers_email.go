package auth

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/session"
)

// VerifyPendingPage renders the "check your email" page.
func (a *App[P]) VerifyPendingPage(w http.ResponseWriter, r *http.Request) error {
	return verifyPendingPage(w, r)
}

// VerifyEmail handles the email verification link.
func (a *App[P]) VerifyEmail(w http.ResponseWriter, r *http.Request) error {
	token := r.URL.Query().Get("token")
	if token == "" {
		return verifyEmailErrorPage(w, r, "missing_token")
	}

	ctx := r.Context()
	tokenHash := HashToken(token)

	verificationToken, err := a.repo.GetEmailVerificationToken(ctx, tokenHash)
	if err != nil {
		return verifyEmailErrorPage(w, r, "invalid_token")
	}

	if time.Now().After(verificationToken.ExpiresAt) {
		if delErr := a.repo.DeleteEmailVerificationToken(ctx, verificationToken.ID); delErr != nil {
			slog.Error("failed to delete expired verification token", "token_id", verificationToken.ID, "error", delErr)
		}
		return verifyEmailErrorPage(w, r, "token_expired")
	}

	if markErr := a.repo.MarkEmailVerified(ctx, verificationToken.UserID); markErr != nil {
		slog.Error("failed to mark email as verified", "error", markErr)
		return verifyEmailErrorPage(w, r, "verification_failed")
	}

	if delErr := a.repo.DeleteUserEmailVerificationTokens(ctx, verificationToken.UserID); delErr != nil {
		slog.Error("failed to delete verification tokens after verify", "user_id", verificationToken.UserID, "error", delErr)
	}

	user, err := a.repo.GetUserByID(ctx, verificationToken.UserID)
	if err != nil {
		slog.Error("failed to get user after verification", "error", err)
		return verifyEmailErrorPage(w, r, "verification_failed")
	}

	if err := session.Save(w, r, map[string]any{"user_id": user.ID}); err != nil {
		slog.Error("failed to create session after verification", "error", err)
		return verifyEmailErrorPage(w, r, "verification_failed")
	}

	return verifyEmailSuccessPage(w, r)
}

// ResendVerificationRequest is the request body for resending verification email.
type ResendVerificationRequest struct {
	Email string `json:"email" form:"email"`
}

// ResendVerification resends the verification email.
func (a *App[P]) ResendVerification(w http.ResponseWriter, r *http.Request) error {
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
