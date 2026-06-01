package auth

import (
	"context"
	"net/http"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/session"
	"golang.org/x/crypto/bcrypt"
)

// RecoveryLoginRequest is the request body for recovery login.
type RecoveryLoginRequest struct {
	Username string `json:"username" form:"username"`
	Code     string `json:"code" form:"code"`
}

// RecoveryPage renders the recovery login page.
func (a *App[P]) RecoveryPage(w http.ResponseWriter, r *http.Request) error {
	return recoveryPage(w, r, a.config.LoginRedirect)
}

// RecoveryLogin authenticates a user with a recovery code.
func (a *App[P]) RecoveryLogin(w http.ResponseWriter, r *http.Request) error {
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

	remaining, err := a.repo.GetUnusedRecoveryCodeCount(r.Context(), user.ID)
	if err != nil {
		return errorJSONLog(w, http.StatusInternalServerError, "failed to count recovery codes", err)
	}

	if remaining == 0 {
		// Last code was just consumed — regenerate and route through the
		// ack flow so the user cannot proceed without a fresh safety net.
		codes, err := a.generateAndStoreRecoveryCodes(r.Context(), user.ID)
		if err != nil {
			return errorJSONLog(w, http.StatusInternalServerError, "failed to regenerate codes", err)
		}
		if err := session.Set(w, r, "recovery_codes", codes); err != nil {
			return errorJSONLog(w, http.StatusInternalServerError, "failed to store recovery codes", err)
		}
		if err := session.Set(w, r, "redirect_after_login", redirect); err != nil {
			return errorJSONLog(w, http.StatusInternalServerError, "failed to store redirect", err)
		}
		return burrow.JSON(w, http.StatusOK, map[string]any{
			"status":   "ok",
			"redirect": "/auth/recovery-codes",
		})
	}

	return burrow.JSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"remaining_codes": remaining,
		"redirect":        redirect,
	})
}

// RegenerateRecoveryCodes generates new recovery codes and invalidates old ones.
func (a *App[P]) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) error {
	user := MustCurrentUser[P](r.Context())
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
func (a *App[P]) RecoveryCodesPage(w http.ResponseWriter, r *http.Request) error {
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

	return recoveryCodesPage(w, r, codes)
}

// AcknowledgeRecoveryCodes clears recovery codes from the session and redirects.
func (a *App[P]) AcknowledgeRecoveryCodes(w http.ResponseWriter, r *http.Request) error {
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

func (a *App[P]) generateAndStoreRecoveryCodes(ctx context.Context, userID string) ([]string, error) {
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
